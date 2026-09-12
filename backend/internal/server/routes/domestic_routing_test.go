package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDomesticRoutesUseOpenAICompatibilityWithoutChangingOtherPlatforms(t *testing.T) {
	for _, platform := range []string{service.PlatformDomestic, service.PlatformOpenAI, service.PlatformGrok, service.PlatformOpencode} {
		require.True(t, isOpenAICompatiblePlatform(platform), platform)
	}
	for _, platform := range []string{service.PlatformAnthropic, service.PlatformGemini, service.PlatformAntigravity, ""} {
		require.False(t, isOpenAICompatiblePlatform(platform), platform)
	}
}
