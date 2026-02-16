package llm

import "context"

type Provider interface {
	Generate(ctx context.Context, query string, matches []string, messageHistory []string) (string, error)
}
