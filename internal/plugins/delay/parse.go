package delay

import (
	"encoding/json"
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/dsl"
)

// We will round-trip the YAML to JSON to get the correct type
func ParseYAML(step dsl.Step) (*DelayPlugin, error) {
	blob, err := json.Marshal(step)
	if err != nil {
		return nil, fmt.Errorf("the YAML step %s is not valid YAML: %w", step.Name, err)
	}

	var dp DelayPlugin
	if err := json.Unmarshal(blob, &dp); err != nil {
		return nil, fmt.Errorf("the YAML step %s could not be parsed into a %s plugin: %w", step.Name, dp.GetType(), err)
	}
	return &dp, nil
}
