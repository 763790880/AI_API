package service

import "context"

// Domestic is a group and pricing scope, not a new upstream wire protocol.
// Keep its group marker separate from ForcePlatform so account scheduling still
// uses the existing OpenAI-compatible transport and group-scoped account bucket.
type domesticGroupContextKey struct{}

func WithDomesticGroupRouting(ctx context.Context, groupID int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, domesticGroupContextKey{}, groupID)
}

func isDomesticUpstreamAccount(account *Account) bool {
	return account != nil && account.Platform == PlatformOpenAI && account.Type == AccountTypeAPIKey &&
		account.OwnerUserID == nil && isThirdPartyUpstreamAccount(account)
}

func domesticAccountAllowed(ctx context.Context, account *Account) bool {
	if ctx == nil {
		return true
	}
	groupID, domestic := ctx.Value(domesticGroupContextKey{}).(int64)
	if !domestic {
		return true
	}
	if groupID <= 0 || !isDomesticUpstreamAccount(account) {
		return false
	}
	if containsInt64(account.GroupIDs, groupID) {
		return true
	}
	for _, binding := range account.AccountGroups {
		if binding.GroupID == groupID {
			return true
		}
	}
	return false
}

func filterDomesticAccounts(ctx context.Context, accounts []Account) []Account {
	if ctx == nil || ctx.Value(domesticGroupContextKey{}) == nil {
		return accounts
	}
	filtered := make([]Account, 0, len(accounts))
	for i := range accounts {
		if domesticAccountAllowed(ctx, &accounts[i]) {
			filtered = append(filtered, accounts[i])
		}
	}
	return filtered
}

// Discovery intersects the bound account whitelist with this group's saved
// domestic pricing. No global model/price fallback is used for the new platform.
func (s *GatewayService) domesticAvailableModels(ctx context.Context, groupID *int64) []string {
	models := []string{}
	if groupID == nil || *groupID <= 0 || s.accountRepo == nil || s.channelService == nil {
		return models
	}
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, *groupID)
	if err != nil {
		return models
	}
	priced, err := s.channelService.ListSelectablePricedModelIDs(ctx, PricedModelQuery{Platform: PlatformDomestic, GroupID: groupID})
	if err != nil {
		return models
	}
	for _, model := range priced {
		mapped := s.channelService.ResolveChannelMapping(ctx, *groupID, model).MappedModel
		for i := range accounts {
			account := &accounts[i]
			if isDomesticUpstreamAccount(account) && account.IsSchedulable() && account.IsModelSupported(mapped) {
				models = append(models, model)
				break
			}
		}
	}
	return models
}
