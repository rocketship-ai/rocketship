package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

type checkResult struct {
	name     string
	ok       bool
	critical bool
	messages []string
}

// NewDoctorCmd creates a doctor subcommand that inspects common CLI issues.
func NewDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose Rocketship CLI environment issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			results := runDoctorChecks()
			out := cmd.OutOrStdout()
			criticalIssues := 0

			for _, res := range results {
				switch {
				case res.ok:
					_, _ = fmt.Fprintf(out, "[PASS] %s\n", res.name)
				case res.critical:
					_, _ = fmt.Fprintf(out, "[FAIL] %s\n", res.name)
					criticalIssues++
				default:
					_, _ = fmt.Fprintf(out, "[WARN] %s\n", res.name)
				}

				for _, msg := range res.messages {
					_, _ = fmt.Fprintf(out, "    %s\n", msg)
				}
			}

			if criticalIssues > 0 {
				return fmt.Errorf("doctor found %d critical issue(s)", criticalIssues)
			}

			_, _ = fmt.Fprintln(out, "All checks passed.")
			return nil
		},
	}

	return cmd
}

func runDoctorChecks() []checkResult {
	execPath, execErr := os.Executable()
	return []checkResult{
		checkPath(execPath, execErr),
		checkConfig(),
		checkLegacyDir(),
		checkQuarantine(execPath, execErr),
	}
}

func checkPath(execPath string, execErr error) checkResult {
	res := checkResult{name: "PATH configuration"}
	if execErr != nil {
		res.ok = false
		res.critical = true
		res.messages = []string{fmt.Sprintf("unable to resolve executable path: %v", execErr)}
		return res
	}

	execDir := filepath.Dir(execPath)
	cleanExecDir := cleanPath(execDir)
	res.messages = append(res.messages, fmt.Sprintf("rocketship resolves to %s", execPath))

	pathEntries := buildPathSet()
	execInPath := pathEntries.contains(cleanExecDir)

	home, _ := os.UserHomeDir()
	var localBin string
	if home != "" {
		localBin = cleanPath(filepath.Join(home, ".local", "bin"))
	}

	if localBin != "" {
		localBinary := filepath.Join(localBin, "rocketship")
		if fileExists(localBinary) && !pathEntries.contains(localBin) {
			res.ok = false
			res.critical = true
			res.messages = append(res.messages,
				"~/.local/bin contains rocketship but is missing from PATH.",
				"Add this to your shell configuration:",
				"  export PATH=\"$HOME/.local/bin:$PATH\"")
			return res
		}
	}

	if !execInPath {
		res.ok = false
		res.critical = true
		res.messages = append(res.messages,
			fmt.Sprintf("%s is not present in PATH. New shells will not find rocketship.", cleanExecDir),
			fmt.Sprintf("Add it with:  export PATH=\"%s:$PATH\"", cleanExecDir))
		return res
	}

	res.ok = true
	return res
}

func checkConfig() checkResult {
	res := checkResult{name: "Config directory permissions"}

	dir, err := platformConfigDir()
	if err != nil {
		res.ok = false
		res.critical = true
		res.messages = []string{fmt.Sprintf("failed to resolve config directory: %v", err)}
		return res
	}

	dir = cleanPath(dir)
	info, statErr := os.Stat(dir)
	if errors.Is(statErr, os.ErrNotExist) {
		res.ok = true
		res.messages = []string{fmt.Sprintf("config directory %s does not exist yet; it will be created on first use.", dir)}
		return res
	}

	if statErr != nil {
		res.ok = false
		res.critical = true
		res.messages = []string{fmt.Sprintf("failed to stat %s: %v", dir, statErr)}
		return res
	}

	var problems []string
	perm := info.Mode().Perm()
	if perm != 0o700 {
		problems = append(problems, fmt.Sprintf("expected permissions 0700 on %s, found %o. Fix with: chmod 700 %s", dir, perm, dir))
	}

	if owned, ownErr := pathOwnedByCurrentUser(dir); ownErr == nil && !owned {
		problems = append(problems, fmt.Sprintf("%s is not owned by the current user. Fix with: sudo chown -R \"$USER\":\"$(id -gn)\" %s", dir, dir))
	}

	if writeErr := verifyWritable(dir); writeErr != nil {
		problems = append(problems, fmt.Sprintf("directory %s is not writable: %v", dir, writeErr))
		problems = append(problems, fmt.Sprintf("Restore permissions with: chmod u+rwX %s", dir))
	}

	configFile := filepath.Join(dir, "config.json")
	if cfgInfo, err := os.Stat(configFile); err == nil {
		cfgPerm := cfgInfo.Mode().Perm()
		if cfgPerm != 0o600 {
			problems = append(problems, fmt.Sprintf("expected permissions 0600 on %s, found %o. Fix with: chmod 600 %s", configFile, cfgPerm, configFile))
		}
		if owned, ownErr := pathOwnedByCurrentUser(configFile); ownErr == nil && !owned {
			problems = append(problems, fmt.Sprintf("%s is not owned by the current user. Fix with: sudo chown $USER %s", configFile, configFile))
		}
	}

	if len(problems) > 0 {
		res.ok = false
		res.critical = true
		res.messages = problems
		return res
	}

	res.ok = true
	res.messages = []string{fmt.Sprintf("config directory %s has correct ownership and permissions", dir)}
	return res
}

func checkLegacyDir() checkResult {
	res := checkResult{name: "Legacy ~/.rocketship directory"}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		res.ok = true
		return res
	}

	legacyDir := filepath.Join(home, ".rocketship")
	info, statErr := os.Stat(legacyDir)
	if errors.Is(statErr, os.ErrNotExist) {
		res.ok = true
		return res
	}

	if statErr != nil {
		res.ok = false
		res.critical = true
		res.messages = []string{fmt.Sprintf("failed to stat %s: %v", legacyDir, statErr)}
		return res
	}

	if info.IsDir() {
		if owned, ownErr := pathOwnedByCurrentUser(legacyDir); ownErr == nil && !owned {
			res.ok = false
			res.critical = true
			res.messages = []string{
				"found legacy ~/.rocketship directory not owned by current user.",
				"Run:",
				"  sudo chown -R \"$USER\":\"$(id -gn)\" \"$HOME/.rocketship\"",
				"  chmod -R u+rwX,go-rwx \"$HOME/.rocketship\"",
			}
			return res
		}
	}

	res.ok = true
	res.messages = []string{"legacy directory, if present, has valid ownership"}
	return res
}

func checkQuarantine(execPath string, execErr error) checkResult {
	res := checkResult{name: "macOS quarantine attribute"}
	if execErr != nil {
		res.ok = true
		return res
	}

	if runtime.GOOS != "darwin" {
		res.ok = true
		return res
	}

	xattrPath, err := exec.LookPath("xattr")
	if err != nil {
		res.ok = true
		return res
	}

	cmd := exec.Command(xattrPath, "-p", "com.apple.quarantine", execPath)
	if err := cmd.Run(); err == nil {
		res.ok = false
		res.critical = true
		res.messages = []string{
			fmt.Sprintf("rocketship binary at %s has the com.apple.quarantine attribute.", execPath),
			"Remove it with:",
			"  xattr -d com.apple.quarantine $(command -v rocketship)",
		}
		return res
	}

	res.ok = true
	res.messages = []string{"no quarantine attribute detected"}
	return res
}

type pathSet map[string]struct{}

func buildPathSet() pathSet {
	entries := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	set := make(pathSet, len(entries))
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		clean := cleanPath(entry)
		set[clean] = struct{}{}
	}
	return set
}

func (p pathSet) contains(target string) bool {
	_, ok := p[target]
	if ok {
		return true
	}
	for existing := range p {
		if samePath(existing, target) {
			return true
		}
	}
	return false
}

func cleanPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func verifyWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".rocketship-doctor")
	if err != nil {
		return err
	}
	path := f.Name()
	_ = f.Close()
	return os.Remove(path)
}
