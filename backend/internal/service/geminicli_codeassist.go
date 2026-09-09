package service

import (
	"context"

	"github.com/your-org/ai-gateway-platform/internal/pkg/geminicli"
)

// GeminiCliCodeAssistClient calls GeminiCli internal Code Assist endpoints.
type GeminiCliCodeAssistClient interface {
	LoadCodeAssist(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error)
	OnboardUser(ctx context.Context, accessToken, proxyURL string, req *geminicli.OnboardUserRequest) (*geminicli.OnboardUserResponse, error)
}
