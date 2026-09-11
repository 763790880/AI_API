package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIUnboundModeGroupSchedulerFallsBack(t *testing.T) {
	groupID := int64(61713)
	svc := &OpenAIGatewayService{
		accountShareModeService: &AccountShareModeService{repo: &accountShareModeRepoStub{}},
	}
	for _, tc := range []struct {
		name         string
		ctx          context.Context
		wantFallback bool
	}{
		{"authenticated without room", WithAccountShareModeRequest(context.Background(), 10, 20), true},
		{"missing identity", context.Background(), false},
		{"invalid user", WithAccountShareModeRequest(context.Background(), 0, 20), false},
		{"invalid key", WithAccountShareModeRequest(context.Background(), 10, 0), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selection, _, handled, err := svc.selectAccountShareModeBoundAccount(tc.ctx, &groupID, "", nil, OpenAIUpstreamTransportAny, "", "", false)
			require.Nil(t, selection)
			require.Equal(t, !tc.wantFallback, handled)
			if tc.wantFallback {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrAccountShareModeGroupUnbound)
			}
		})
	}
}

func TestOpenAIUnboundModeGroupDispatchAllowsOnlyOrdinaryAdminAccount(t *testing.T) {
	groupID, ownerID, listingID := int64(61713), int64(10), int64(30)
	for _, tc := range []struct {
		name    string
		owner   *int64
		listing *int64
		allowed bool
	}{
		{"ordinary admin account", nil, nil, true},
		{"private account even for owner", &ownerID, nil, false},
		{"room account", nil, &listingID, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{ID: 41, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 3, GroupIDs: []int64{groupID}}
			latest := account
			latest.OwnerUserID, latest.AccountShareModeListingID = tc.owner, tc.listing
			svc := &OpenAIGatewayService{
				accountRepo:             stubOpenAIAccountRepo{accounts: []Account{latest}},
				accountShareModeService: &AccountShareModeService{repo: &accountShareModeRepoStub{}},
			}
			ctx := WithAccountShareModeRequest(context.Background(), ownerID, 20)
			result, err := svc.RevalidateSelectedOpenAIAccountForDispatch(ctx, &groupID, &account, OpenAIAccountDispatchRequirements{RequiredTransport: OpenAIUpstreamTransportAny})
			if tc.allowed {
				require.NoError(t, err)
				require.NotNil(t, result)
			} else {
				require.ErrorIs(t, err, ErrAccountShareModeGroupUnbound)
				require.Nil(t, result)
			}
		})
	}
}

func TestOpenAIUnboundModeGroupDoesNotFallbackOnRoomFailure(t *testing.T) {
	groupID := int64(61713)
	for _, roomErr := range []error{ErrAccountShareMembershipIdleTimeout, ErrAccountShareMembershipEnding, ErrAccountShareModeRecovering, ErrServiceUnavailable} {
		t.Run(roomErr.Error(), func(t *testing.T) {
			svc := &OpenAIGatewayService{accountShareModeService: &AccountShareModeService{repo: &accountShareModeRepoStub{}}}
			ctx := WithAccountShareModeRequest(context.Background(), 10, 20)
			request, _ := AccountShareModeRequestFromContext(ctx)
			request.state.set(10, 20, groupID, nil, nil, roomErr)
			selection, _, handled, err := svc.selectAccountShareModeBoundAccount(ctx, &groupID, "", nil, OpenAIUpstreamTransportAny, "", "", false)
			require.Nil(t, selection)
			require.True(t, handled)
			require.ErrorIs(t, err, roomErr)
		})
	}
}
