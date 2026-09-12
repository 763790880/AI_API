package service

import (
	"testing"
	"time"
)

func TestAccountSchedulableInQuotaGroup_ThirdPartyUsesBoundGroupPlatform(t *testing.T) {
	now := time.Now()
	account := Account{
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Schedulable: true,
		Extra:       map[string]any{"account_source": "third_party"},
	}
	if !accountSchedulableInQuotaGroup(account, now, StatusActive, "kiro", "", false, false) {
		t.Fatal("third-party account should be schedulable in its bound group platform")
	}
}

func TestAccountSchedulableInQuotaGroup_RegularPlatformMismatchRemainsUnavailable(t *testing.T) {
	now := time.Now()
	account := Account{Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true}
	if accountSchedulableInQuotaGroup(account, now, StatusActive, "kiro", "", false, false) {
		t.Fatal("regular account with mismatched platform must remain unavailable")
	}
}
