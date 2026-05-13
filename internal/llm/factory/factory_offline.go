//go:build offline

package factory

import (
	"context"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/llm"
	"github.com/akolanti/GoAPI/internal/llm/local"
)

func NewProvider(ctx context.Context) llm.Provider {
	return local.GetLocalClient(ctx, config.LLMModelName)
}
