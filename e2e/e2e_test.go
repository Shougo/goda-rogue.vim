// Package e2e runs the game in a real Vim and Neovim: the scenario
// testdata/test.vim, with the binary of the repository.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// editor is the executable of Vim or Neovim: $GODA_VIM or $GODA_NVIM, or
// the one in PATH.
func editor(t *testing.T, env, name string) string {
	t.Helper()
	if p := os.Getenv(env); p != "" {
		return p
	}
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("no %s", name)
	}
	return p
}

// setup builds the binary, and returns the environment of the scenario.
func setup(t *testing.T) []string {
	t.Helper()
	if testing.Short() {
		t.Skip("-short")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "goda-rogue")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, "..").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/Shougo/goda.vim").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return append(os.Environ(),
		"GODA_VIM_DIR="+strings.TrimSpace(string(out)),
		"GODA_ROGUE_DIR="+root,
		"GODA_ROGUE_BINARY="+bin,
		"GODA_ROGUE_RESULT="+filepath.Join(dir, "result"),
	)
}

func run(t *testing.T, env []string, name string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	var result string
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "GODA_ROGUE_RESULT="); ok {
			result = v
		}
	}
	b, rerr := os.ReadFile(result)
	if rerr != nil {
		t.Fatalf("no result (%v): %v\n%s", err, rerr, out)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if ctx.Err() != nil {
		// The checks may have passed, but the editor did not quit: a
		// prompt left waiting for an answer, say.
		t.Errorf("%s did not quit in time", name)
	}
	for _, l := range lines {
		if l == "ok" {
			return
		}
	}
	t.Errorf("%s", strings.Join(lines, "\n"))
}

func script(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("testdata/test.vim")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVim(t *testing.T) {
	vim := editor(t, "GODA_VIM", "vim")
	env := setup(t)
	run(t, env, vim, "-N", "-u", "NONE", "-i", "NONE", "-n", "-es", "-S", script(t))
}

func TestNeovim(t *testing.T) {
	nvim := editor(t, "GODA_NVIM", "nvim")
	env := setup(t)
	run(t, env, nvim, "--headless", "--clean", "-n", "-S", script(t))
}
