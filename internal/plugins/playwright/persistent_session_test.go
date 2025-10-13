package playwright

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rocketship-ai/rocketship/internal/browser/sessionfile"
	browseruse "github.com/rocketship-ai/rocketship/internal/plugins/browser_use"
)

func TestPersistentSessionFlowWithStubs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub python script uses POSIX shell semantics")
	}

	ctx := context.Background()

	stubDir := t.TempDir()
	stubPath := filepath.Join(stubDir, "python3")
	stubScript := "#!/usr/bin/env bash\n" +
		"script=\"$1\"\n" +
		"shift\n" +
		"case \"$(basename \"$script\")\" in\n" +
		"  playwright_runner.py)\n" +
		"    mode=\"$1\"\n" +
		"    shift\n" +
		"    if [ \"$mode\" = \"start\" ]; then\n" +
		"      echo '{\"ok\": true, \"wsEndpoint\": \"ws://fake\"}'\n" +
		"      while true; do sleep 1; done\n" +
		"    elif [ \"$mode\" = \"script\" ]; then\n" +
		"      echo '{\"ok\": true, \"result\": {\"script\": \"ran\"}}'\n" +
		"    elif [ \"$mode\" = \"stop\" ]; then\n" +
		"      echo '{\"ok\": true}'\n" +
		"    else\n" +
		"      echo '{\"ok\": false, \"error\": \"unknown mode\"}'\n" +
		"      exit 1\n" +
		"    fi\n" +
		"    ;;\n" +
		"  browser_use_runner.py)\n" +
		"    echo '{\"ok\": true, \"result\": {\"agent\": \"ran\"}}'\n" +
		"    ;;\n" +
		"  *)\n" +
		"    echo '{\"ok\": false, \"error\": \"unknown script\"}'\n" +
		"    exit 1\n" +
		"    ;;\n" +
		"esac\n"

	// Guard against Python embedding printing bytecode headers; ensure we always
	// produce exactly one JSON line followed by EOF.
	if err := os.WriteFile(stubPath, []byte(stubScript), 0o755); err != nil {
		t.Fatalf("failed to write stub python: %v", err)
	}

	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	runDir := t.TempDir()
	t.Setenv("ROCKETSHIP_RUN_DIR", runDir)

	playPlugin := &Plugin{}

	startParams := map[string]interface{}{
		"config": map[string]interface{}{
			"role":       "start",
			"session_id": "test-session",
			"headless":   true,
		},
	}

	if _, err := playPlugin.Activity(ctx, startParams); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Give stub time to enter sleep loop
	time.Sleep(50 * time.Millisecond)

	scriptParams := map[string]interface{}{
		"config": map[string]interface{}{
			"role":       "script",
			"session_id": "test-session",
			"language":   "python",
			"script":     "page.goto('https://example.com')",
		},
	}

	scriptResultRaw, err := playPlugin.Activity(ctx, scriptParams)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}

	scriptResult, ok := scriptResultRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected script result type: %T", scriptResultRaw)
	}
	if success, _ := scriptResult["success"].(bool); !success {
		t.Fatalf("script returned unsuccessful payload: %#v", scriptResult)
	}

	browserPlugin := &browseruse.Plugin{}

	browserParams := map[string]interface{}{
		"config": map[string]interface{}{
			"session_id":      "test-session",
			"task":            "do something",
			"allowed_domains": []interface{}{"example.com"},
			"max_steps":       5,
		},
	}

	browserResultRaw, err := browserPlugin.Activity(ctx, browserParams)
	if err != nil {
		t.Fatalf("browser_use failed: %v", err)
	}

	browserResult, ok := browserResultRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected browser result type: %T", browserResultRaw)
	}
	if success, _ := browserResult["success"].(bool); !success {
		t.Fatalf("browser_use returned unsuccessful payload: %#v", browserResult)
	}

	stopParams := map[string]interface{}{
		"config": map[string]interface{}{
			"role":       "stop",
			"session_id": "test-session",
		},
	}

	if _, err := playPlugin.Activity(ctx, stopParams); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if _, _, err := sessionfile.Read(ctx, "test-session"); err == nil {
		t.Fatalf("expected session file to be removed after stop")
	}
}
