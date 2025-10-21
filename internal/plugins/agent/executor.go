package agent

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rocketship-ai/rocketship/internal/browser/sessionfile"
)

//go:embed agent_executor.py
var embeddedExecutor []byte

var (
	executorOnce sync.Once
	executorPath string
	executorErr  error
)

// prepareExecutorScript extracts the embedded Python script to a temp directory
// and returns the path to it. The script is extracted once and reused.
func prepareExecutorScript() (string, func(), error) {
	executorOnce.Do(func() {
		baseDir, err := sessionfile.BaseDir()
		if err != nil {
			executorErr = err
			return
		}

		targetDir := filepath.Join(baseDir, "tmp", "agent")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			executorErr = fmt.Errorf("failed to create agent executor dir: %w", err)
			return
		}

		path := filepath.Join(targetDir, "agent_executor.py")
		if err := os.WriteFile(path, embeddedExecutor, 0o700); err != nil {
			executorErr = fmt.Errorf("failed to write agent executor: %w", err)
			return
		}

		executorPath = path
	})

	if executorErr != nil {
		return "", func() {}, executorErr
	}

	return executorPath, func() {}, nil
}
