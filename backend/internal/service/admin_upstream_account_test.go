//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminUpstreamProtocolAndOwnedBoundary(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			credentials := map[string]any{"base_url": "https://relay.example.com/api", "api_key": "test-placeholder"}
			account := &Account{Platform: platform, Type: AccountTypeAPIKey, Credentials: credentials}
			svc := &AccountTestService{cfg: &config.Config{}}
			req, err := svc.buildUpstreamModelsRequest(context.Background(), account)
			require.NoError(t, err)
			require.Equal(t, "https://relay.example.com/api/v1/models", req.URL.String())
			if platform == PlatformOpenAI {
				require.Equal(t, "Bearer test-placeholder", req.Header.Get("Authorization"))
			} else {
				require.Equal(t, "test-placeholder", req.Header.Get("x-api-key"))
				require.Equal(t, "2023-06-01", req.Header.Get("anthropic-version"))
			}
			for _, accountType := range []string{AccountTypeAPIKey, AccountTypeUpstream} {
				require.ErrorIs(t, validateOwnedAccountSourceForPlatform(platform, accountType, credentials, nil), ErrOwnedAccountTypeNotAllowed)
			}
			require.ErrorIs(t, validateOwnedAccountSourceForPlatform(platform, AccountTypeOAuth,
				map[string]any{"access_token": "test-placeholder", "base_url": "https://relay.example.com"}, nil), ErrOwnedAccountCredentialsNotAllowed)
		})
	}
}

func TestAdminUpstreamAnthropicUsesCustomURLWithoutPassthrough(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	account := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://relay.example.com/api", "api_key": "test-placeholder",
	}, Extra: map[string]any{"account_source": "third_party"}}
	svc := &GatewayService{cfg: &config.Config{}}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account,
		[]byte(`{"model":"claude-sonnet-5","messages":[{"role":"user","content":"hello"}]}`),
		"test-placeholder", "api_key", "claude-sonnet-5", false, false)
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/api/v1/messages?beta=true", req.URL.String())
	require.Equal(t, "test-placeholder", getHeaderRaw(req.Header, "x-api-key"))
}

func TestAdminUpstreamOpenAIForwardsCustomURLAndStreamUsage(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set("Authorization", "Bearer inbound-placeholder")
	c.Request.Header.Set("Cookie", "inbound-placeholder")
	upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
		StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: {\"id\":\"chat-test\",\"model\":\"gpt-test\",\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n")),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := &Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://relay.example.com/api", "api_key": "upstream-placeholder",
		"model_mapping": map[string]any{"gpt-test": "gpt-test"},
	}, Extra: map[string]any{"account_source": "third_party"}}
	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, account,
		[]byte(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`), "")
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/api/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer upstream-placeholder", upstream.lastReq.Header.Get("Authorization"))
	require.Empty(t, upstream.lastReq.Header.Get("Cookie"))
	require.Contains(t, recorder.Body.String(), "hello")
	require.Contains(t, recorder.Body.String(), "[DONE]")
	require.Equal(t, 3, result.Usage.InputTokens)
	require.Equal(t, 2, result.Usage.OutputTokens)
}
