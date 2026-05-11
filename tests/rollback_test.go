package tests

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// pswLogSHAs returns short SHAs from `psw log` output, oldest first (matches
// the command's own order).
func pswLogSHAs(t *testing.T, vault string, env map[string]string) []string {
	t.Helper()
	r := runPswEnv(t, vault, env, "log")
	mustExit(t, r, 0)
	var shas []string
	for _, line := range strings.Split(strings.TrimSpace(r.stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			shas = append(shas, fields[0])
		}
	}
	return shas
}

func TestRollback_NoGitRepo(t *testing.T) {
	t.Parallel()
	vault := newVault(t) // PSW_GIT=0
	result := runPsw(t, vault, "rollback")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "Storage is not a git repository")
}

func TestRollback_NoPriorCommits(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	// Only the initial-main-password commit exists, and it's HEAD.
	result := runPswEnv(t, vault, env, "rollback")
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "No previous commits to roll back to")
}

func TestRollback_NoTTY(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p"), 0)
	result := runPswEnv(t, vault, env, "rollback")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "requires an interactive terminal")
}

func TestRollback_PartialEnvVarsRejected(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p"), 0)
	env2 := withEnv(env, "PSW_ROLLBACK_TARGET", "deadbee")
	result := runPswEnv(t, vault, env2, "rollback")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "must be set together")
}

func TestRollback_UnknownTargetSHA(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p"), 0)
	env2 := withEnv(env, "PSW_ROLLBACK_TARGET", "deadbee")
	env3 := withEnv(env2, "PSW_ROLLBACK_YES", "1")
	result := runPswEnv(t, vault, env3, "rollback")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "no commit with short-sha deadbee")
}

func TestRollback_RestoresSnapshot(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p1"), 0)

	// Oldest first — newest commit is at the end.
	shas := pswLogSHAs(t, vault, env)
	if len(shas) < 2 {
		t.Fatalf("expected ≥2 commits after add alpha, got %d:\n%v", len(shas), shas)
	}
	targetSHA := shas[len(shas)-1] // the alpha-add commit

	mustExit(t, runPswEnv(t, vault, env, "add", "beta", "-u", "u", "--password=p2"), 0)
	mustExit(t, runPswEnv(t, vault, env, "add", "gamma", "-u", "u", "--password=p3"), 0)

	rbEnv := withEnv(env, "PSW_ROLLBACK_TARGET", targetSHA)
	rbEnv = withEnv(rbEnv, "PSW_ROLLBACK_YES", "1")
	result := runPswEnv(t, vault, rbEnv, "rollback")
	mustExit(t, result, 0)
	mustContain(t, result.stdout, "Rolled back to "+targetSHA)

	list := runPswEnv(t, vault, env)
	mustExit(t, list, 0)
	mustContain(t, list.stdout, "alpha")
	mustNotContain(t, list.stdout, "beta")
	mustNotContain(t, list.stdout, "gamma")

	logRes := runPswEnv(t, vault, env, "log")
	mustExit(t, logRes, 0)
	mustContain(t, logRes.stdout, "rollback to "+targetSHA)
}

func TestRollback_ChangeMainRefused(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p"), 0)
	shas := pswLogSHAs(t, vault, env)
	preChangeMainSHA := shas[len(shas)-1] // alpha-add SHA, encrypted under defaultMainPassword

	// Rotate main password — re-encrypts everything under the new password.
	envChange := withEnv(env, "PSW_NEW_MAIN_PASSWORD", "rotated")
	mustExit(t, runPswEnv(t, vault, envChange, "change", "main"), 0)

	// Subsequent invocations use the new password.
	envNew := withEnv(env, "PSW_MAIN_PASSWORD", "rotated")
	envNew = withEnv(envNew, "PSW_ROLLBACK_TARGET", preChangeMainSHA)
	envNew = withEnv(envNew, "PSW_ROLLBACK_YES", "1")
	result := runPswEnv(t, vault, envNew, "rollback")
	mustExit(t, result, 1)
	mustContain(t, result.stdout, "encrypted under a previous main password")

	// Vault should be untouched by the failed rollback.
	envNewList := withEnv(env, "PSW_MAIN_PASSWORD", "rotated")
	list := runPswEnv(t, vault, envNewList)
	mustExit(t, list, 0)
	mustContain(t, list.stdout, "alpha")
}

func TestRollback_StampsMTimesForLWW(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "alpha", "-u", "u", "--password=fromA"), 0)

	// B clones, confirms it has alpha synced.
	vaultB, envB := addPeer(t, bare)
	listB1 := runPswEnv(t, vaultB, envB)
	mustExit(t, listB1, 0)
	mustContain(t, listB1.stdout, "alpha")

	// Capture the alpha-only SHA on A before adding beta.
	shasA := pswLogSHAs(t, vaultA, envA)
	preBetaSHA := shasA[len(shasA)-1]

	mustExit(t, runPswEnv(t, vaultA, envA, "add", "beta", "-u", "u", "--password=fromA"), 0)

	// A rolls back to preBetaSHA (drops beta), auto-pushes.
	rbEnv := withEnv(envA, "PSW_ROLLBACK_TARGET", preBetaSHA)
	rbEnv = withEnv(rbEnv, "PSW_ROLLBACK_YES", "1")
	mustExit(t, runPswEnv(t, vaultA, rbEnv, "rollback"), 0)

	// B's next mutation pulls A's rollback; LWW picks A's stamped records.
	time.Sleep(mtimeSeparation)
	mustExit(t, runPswEnv(t, vaultB, envB, "add", "gamma", "-u", "u", "--password=p"), 0)

	listB2 := runPswEnv(t, vaultB, envB)
	mustExit(t, listB2, 0)
	mustContain(t, listB2.stdout, "alpha")
	mustContain(t, listB2.stdout, "gamma")
	if strings.Contains(listB2.stdout, "beta") {
		t.Fatalf("expected beta to be dropped on B after A's rollback; got:\n%s", listB2.stdout)
	}
}

func TestRollback_LogColoring(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	vault, env := newGitVault(t)
	mustExit(t, runPswEnv(t, vault, env, "add", "alpha", "-u", "u", "--password=p"), 0)
	shas := pswLogSHAs(t, vault, env)
	targetSHA := shas[len(shas)-1]
	mustExit(t, runPswEnv(t, vault, env, "add", "beta", "-u", "u", "--password=p"), 0)

	rbEnv := withEnv(env, "PSW_ROLLBACK_TARGET", targetSHA)
	rbEnv = withEnv(rbEnv, "PSW_ROLLBACK_YES", "1")
	mustExit(t, runPswEnv(t, vault, rbEnv, "rollback"), 0)

	// Don't strip ANSI for this check — we want to see the color escape.
	cmd := exec.Command(pswBinary, "log")
	cmd.Env = flatEnvForVault(vault, env)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("psw log: %v", err)
	}
	raw := string(out)
	if !strings.Contains(raw, "\x1b[35m") {
		t.Fatalf("expected purple ANSI escape (\\x1b[35m) in psw log output, got:\n%s", raw)
	}
	if !strings.Contains(raw, "rollback to "+targetSHA) {
		t.Fatalf("expected rollback line for %s in psw log:\n%s", targetSHA, raw)
	}
}

// flatEnvForVault mirrors runPswEnv's env construction but returns the slice
// so callers running their own exec.Command can reuse it. Defaults match
// runPswEnv (PSW_GIT=0); extraEnv overrides.
func flatEnvForVault(vault string, extraEnv map[string]string) []string {
	env := map[string]string{
		"PSW_HOME":          vault,
		"PSW_MAIN_PASSWORD": defaultMainPassword,
		"PSW_GIT":           "0",
		"PATH":              os.Getenv("PATH"),
		"HOME":              os.Getenv("HOME"),
	}
	for k, v := range extraEnv {
		env[k] = v
	}
	flat := make([]string, 0, len(env))
	for k, v := range env {
		flat = append(flat, k+"="+v)
	}
	return flat
}
