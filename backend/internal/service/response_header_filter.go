package service

import (
	"github.com/your-org/ai-gateway-platform/internal/config"
	"github.com/your-org/ai-gateway-platform/internal/util/responseheaders"
)

func compileResponseHeaderFilter(cfg *config.Config) *responseheaders.CompiledHeaderFilter {
	if cfg == nil {
		return nil
	}
	return responseheaders.CompileHeaderFilter(cfg.Security.ResponseHeaders)
}
