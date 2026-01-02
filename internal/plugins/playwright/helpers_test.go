package playwright

import "testing"

func TestProcessAssertions_TemplatesExpectedWithEnv(t *testing.T) {
	// dsl.ProcessTemplate prefers OS env over TemplateContext.Env; CI often sets API_KEY,
	// so pin it to a deterministic value for this test.
	t.Setenv("API_KEY", "secret123")

	params := map[string]interface{}{
		"assertions": []interface{}{
			map[string]interface{}{
				"type":     "json_path",
				"path":     ".foo",
				"expected": "value={{ .env.API_KEY }}",
			},
		},
	}

	result := map[string]interface{}{
		"foo": "value=secret123",
	}

	state := map[string]interface{}{}
	env := map[string]string{"API_KEY": "secret123"}

	if err := processAssertions(params, result, state, env); err != nil {
		t.Fatalf("expected assertions to pass, got error: %v", err)
	}
}
