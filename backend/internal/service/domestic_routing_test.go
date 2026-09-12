//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func domesticTestAccount(id, groupID int64) Account {
	return Account{ID: id, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive,
		Schedulable: true, GroupIDs: []int64{groupID}, Extra: map[string]any{"account_source": "third_party"}}
}

type domesticAccountRepoStub struct {
	AccountRepository
	accounts []Account
	groupID  int64
	platform string
}

func (r *domesticAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]Account, error) {
	r.groupID, r.platform = groupID, platform
	return r.accounts, nil
}

func TestDomesticRoutingUsesBoundAdminAPIKeysOnly(t *testing.T) {
	groupID := int64(42)
	allowed := domesticTestAccount(1, groupID)
	oauth := allowed
	oauth.ID, oauth.Type = 2, AccountTypeOAuth
	owned := allowed
	ownerID := int64(7)
	owned.ID, owned.OwnerUserID = 3, &ownerID
	otherGroup := domesticTestAccount(4, 99)
	repo := &domesticAccountRepoStub{accounts: []Account{allowed, oauth, owned, otherGroup}}
	svc := &OpenAIGatewayService{accountRepo: repo}
	ctx := WithDomesticGroupRouting(context.Background(), groupID)
	accounts, err := svc.listSchedulableAccounts(ctx, &groupID)
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, repo.platform)
	require.Equal(t, groupID, repo.groupID)
	require.Equal(t, []Account{allowed}, accounts)
	// Production's snapshot path applies exactly the same group/owner policy.
	svc.schedulerSnapshot = &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{
		snapshotAccounts: []*Account{&allowed, &oauth, &owned, &otherGroup},
	}}
	accounts, err = svc.listSchedulableAccounts(ctx, &groupID)
	require.NoError(t, err)
	require.Equal(t, []Account{allowed}, accounts)
	svc.schedulerSnapshot = nil
	// Ordinary requests keep the original eligibility path.
	require.True(t, domesticAccountAllowed(context.Background(), &oauth))
	require.False(t, domesticAccountAllowed(WithDomesticGroupRouting(ctx, 0), &allowed))
	// A stale sticky selection cannot bypass the domestic policy; its slot is released.
	released := false
	_, err = svc.newSelectionResult(ctx, &oauth, true, func() { released = true }, nil)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.True(t, released)
	require.True(t, accountSchedulableInQuotaGroup(allowed, time.Now(), StatusActive, PlatformDomestic, "", false, false))
	require.False(t, accountSchedulableInQuotaGroup(oauth, time.Now(), StatusActive, PlatformDomestic, "", false, false))
}

func TestDomesticModelsAndPricingRemainGroupScoped(t *testing.T) {
	domesticID, openaiID := int64(42), int64(43)
	account := domesticTestAccount(1, domesticID)
	account.Credentials = map[string]any{"model_mapping": map[string]any{"glm-5.3": "glm-5.3", "unpriced": "unpriced"}}
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{domesticID: {account}}}
	channels := newTestChannelService(&mockChannelRepository{
		getGroupPlatformsFn: func(context.Context, []int64) (map[int64]string, error) {
			return map[int64]string{domesticID: PlatformDomestic, openaiID: PlatformOpenAI}, nil
		},
		listAllFn: func(context.Context) ([]Channel, error) {
			return []Channel{
				{ID: 1, Status: StatusActive, GroupIDs: []int64{domesticID}, ModelPricing: []ChannelModelPricing{
					{Platform: PlatformDomestic, Models: []string{"glm-5.3"}, InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(8e-6)},
					{Platform: PlatformDomestic, Models: []string{"unsupported"}},
				}},
				{ID: 2, Status: StatusActive, GroupIDs: []int64{openaiID}, ModelPricing: []ChannelModelPricing{
					{Platform: PlatformOpenAI, Models: []string{"glm-5.3"}, InputPrice: testPtrFloat64(5e-6), OutputPrice: testPtrFloat64(15e-6)},
					{Platform: PlatformOpenAI, Models: []string{"unpriced"}},
				}},
			}, nil
		},
	})
	svc := &GatewayService{accountRepo: repo, channelService: channels}
	ctx := context.Background()
	require.Equal(t, []string{"glm-5.3"}, svc.GetAvailableModels(ctx, &domesticID, PlatformDomestic))
	require.Empty(t, svc.GetAvailableModels(ctx, nil, PlatformDomestic))
	emptyGroup := int64(999)
	require.Empty(t, svc.GetAvailableModels(ctx, &emptyGroup, PlatformDomestic))
	// The same model keeps different saved rates in domestic/OpenAI channels.
	resolver := NewModelPricingResolver(channels, newTestBillingServiceForResolver())
	for _, tc := range []struct {
		groupID       int64
		input, output float64
	}{
		{domesticID, 2e-6, 8e-6}, {openaiID, 5e-6, 15e-6},
	} {
		resolved := resolver.Resolve(ctx, PricingInput{Model: "glm-5.3", GroupID: &tc.groupID})
		price := resolver.GetIntervalPricing(resolved, 1000)
		require.NotNil(t, price)
		require.Equal(t, tc.input, price.InputPricePerToken)
		require.Equal(t, tc.output, price.OutputPricePerToken)
	}
}

func TestDomesticQwenTiersUseExistingBillingBoundaries(t *testing.T) {
	const fx = 6.7082
	resolver := NewModelPricingResolver(nil, nil)
	pricing := &ResolvedPricing{Mode: BillingModeToken, Intervals: []PricingInterval{
		{MinTokens: 0, MaxTokens: testPtrInt(256000), InputPrice: testPtrFloat64(2 / fx / 1e6), OutputPrice: testPtrFloat64(8 / fx / 1e6)},
		{MinTokens: 256000, InputPrice: testPtrFloat64(6 / fx / 1e6), OutputPrice: testPtrFloat64(24 / fx / 1e6)},
	}}
	for _, tc := range []struct {
		tokens        int
		input, output float64
	}{
		{1, 2, 8}, {256000, 2, 8}, {256001, 6, 24}, {1000000, 6, 24},
	} {
		resolved := resolver.GetIntervalPricing(pricing, tc.tokens)
		require.InDelta(t, tc.input, resolved.InputPricePerToken*1e6*fx, 1e-8)
		require.InDelta(t, tc.output, resolved.OutputPricePerToken*1e6*fx, 1e-8)
	}
}
