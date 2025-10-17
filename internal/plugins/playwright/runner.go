package playwright

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rocketship-ai/rocketship/internal/browser/sessionfile"
)

//go:embed playwright_runner.py
var embeddedRunner []byte

var (
	runnerOnce sync.Once
	runnerPath string
	runnerErr  error
)

func prepareRunnerScript() (string, func(), error) {
	runnerOnce.Do(func() {
		baseDir, err := sessionfile.BaseDir()
		if err != nil {
			runnerErr = err
			return
		}

		targetDir := filepath.Join(baseDir, "tmp", "playwright")
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			runnerErr = fmt.Errorf("failed to create playwright runner dir: %w", err)
			return
		}

		path := filepath.Join(targetDir, "playwright_runner.py")
		if err := os.WriteFile(path, embeddedRunner, 0o700); err != nil {
			runnerErr = fmt.Errorf("failed to write playwright runner: %w", err)
			return
		}

		runnerPath = path
	})

	if runnerErr != nil {
		return "", func() {}, runnerErr
	}

	return runnerPath, func() {}, nil
}
