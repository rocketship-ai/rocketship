package interpreter

import (
	"context"
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/dsl"
)

type TemplateResolveInput struct {
	Template string            `json:"template"`
	Runtime  map[string]string `json:"runtime"`
	Env      map[string]string `json:"env"`
}

func TemplateResolverActivity(ctx context.Context, input TemplateResolveInput) (string, error) {
	runtime := make(map[string]interface{}, len(input.Runtime))
	for key, value := range input.Runtime {
		runtime[key] = value
	}

	out, err := dsl.ProcessTemplate(input.Template, dsl.TemplateContext{
		Runtime: runtime,
		Env:     input.Env,
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve template: %w", err)
	}
	return out, nil
}
