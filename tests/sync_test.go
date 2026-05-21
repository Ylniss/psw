package tests

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mtimeSeparation ensures two psw invocations get distinct UnixMilli stamps.
const mtimeSeparation = 20 * time.Millisecond

// TestSync_AutoPushOnAdd: a mutating command pushes its commit to bare.
func TestSync_AutoPushOnAdd(t *testing.T) {
	t.Parallel()
	vault, bare, env := newGitVaultWithRemote(t)
	beforeCount := bareCommitCount(t, bare)

	mustExit(t, runPswEnv(t, vault, env, "add", "foo", "-u", "u", "--password=p"), 0)

	afterCount := bareCommitCount(t, bare)
	if afterCount <= beforeCount {
		t.Fatalf("expected commit count to increase after add; before=%d after=%d", beforeCount, afterCount)
	}
	log := runGitInBare(t, bare, "log", "--format=%s")
	if !strings.Contains(log, "added new record") {
		t.Fatalf("bare log missing 'added new record':\n%s", log)
	}
}

// TestSync_AutoPullBeforeMutate: vault A adds; vault B's next mutation pulls A's record first.
func TestSync_AutoPullBeforeMutate(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "alpha", "-u", "u", "--password=p"), 0)

	vaultB, envB := addPeer(t, bare)
	mustExit(t, runPswEnv(t, vaultB, envB, "add", "beta", "-u", "u", "--password=p"), 0)

	assertVaultHasRecords(t, vaultB, envB, "alpha", "beta")
}

// TestSync_SmartMerge_DisjointAdds: both peers add disjoint records while offline,
// then come back online; both ends converge.
func TestSync_SmartMerge_DisjointAdds(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	vaultB, envB := addPeer(t, bare)

	offlineEnvA := withEnv(envA, "PSW_GIT_REMOTE", "0")
	offlineEnvB := withEnv(envB, "PSW_GIT_REMOTE", "0")

	mustExit(t, runPswEnv(t, vaultA, offlineEnvA, "add", "alpha", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vaultB, offlineEnvB, "add", "beta", "-u", "u", "--password=p"), 0)

	// A re-enables remote, push goes through.
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "gamma", "-u", "u", "--password=p"), 0)
	// B re-enables remote, pre-pull merges A's alpha+gamma with B's beta.
	mustExit(t, runPswEnv(t, vaultB, envB, "add", "delta", "-u", "u", "--password=p"), 0)

	assertVaultHasRecords(t, vaultB, envB, "alpha", "beta", "gamma", "delta")

	// One more mutation on A pulls B's beta+delta.
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "epsilon", "-u", "u", "--password=p"), 0)
	assertVaultHasRecords(t, vaultA, envA, "alpha", "beta", "gamma", "delta", "epsilon")
}

// TestSync_SmartMerge_SameRecordNewestWins: both peers add same name offline; later mtime wins.
func TestSync_SmartMerge_SameRecordNewestWins(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	vaultB, envB := addPeer(t, bare)

	offlineEnvA := withEnv(envA, "PSW_GIT_REMOTE", "0")
	offlineEnvB := withEnv(envB, "PSW_GIT_REMOTE", "0")

	mustExit(t, runPswEnv(t, vaultA, offlineEnvA, "add", "shared", "-u", "u", "--password=fromA"), 0)
	time.Sleep(mtimeSeparation)
	mustExit(t, runPswEnv(t, vaultB, offlineEnvB, "add", "shared", "-u", "u", "--password=fromB"), 0)

	// A pushes via a follow-up mutation.
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "anchorA", "-u", "u", "--password=p"), 0)
	// B re-enables remote and triggers pull-merge-push.
	mustExit(t, runPswEnv(t, vaultB, envB, "add", "anchorB", "-u", "u", "--password=p"), 0)

	// B's "shared" should still be fromB (B was newer).
	resB := runPswEnv(t, vaultB, envB, "get", "shared", "--exact", "--stdout")
	mustExit(t, resB, 0)
	mustEqual(t, trimmed(resB), "fromB")

	// A pulls and converges.
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "anchorA2", "-u", "u", "--password=p"), 0)
	resA := runPswEnv(t, vaultA, envA, "get", "shared", "--exact", "--stdout")
	mustExit(t, resA, 0)
	mustEqual(t, trimmed(resA), "fromB")
}

// TestSync_SmartMerge_RemovedOnOneSide: removal with no concurrent edit wins.
func TestSync_SmartMerge_RemovedOnOneSide(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "alpha", "-u", "u", "--password=p"), 0)

	vaultB, envB := addPeer(t, bare)
	assertVaultHasRecords(t, vaultB, envB, "alpha")

	mustExit(t, runPswEnv(t, vaultA, envA, "remove", "alpha", "-e"), 0)
	// B's next mutation pulls and merges → alpha drops.
	mustExit(t, runPswEnv(t, vaultB, envB, "add", "beta", "-u", "u", "--password=p"), 0)

	assertVaultLacksRecords(t, vaultB, envB, "alpha")
	assertVaultHasRecords(t, vaultB, envB, "beta")
}

// TestSync_SmartMerge_RemovedVsModified: modification beats removal when L.mtime > F.mtime.
func TestSync_SmartMerge_RemovedVsModified(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "alpha", "-u", "u", "--password=p0"), 0)

	vaultB, envB := addPeer(t, bare)
	// Confirm B has alpha.
	assertVaultHasRecords(t, vaultB, envB, "alpha")

	// Both go offline so neither pushes immediately.
	offlineEnvA := withEnv(envA, "PSW_GIT_REMOTE", "0")
	offlineEnvB := withEnv(envB, "PSW_GIT_REMOTE", "0")

	// A removes alpha offline.
	mustExit(t, runPswEnv(t, vaultA, offlineEnvA, "remove", "alpha", "-e"), 0)
	// B modifies alpha offline (mtime now > fork mtime).
	time.Sleep(mtimeSeparation)
	mustExit(t, runPswEnv(t, vaultB, offlineEnvB, "change", "alpha", "--password=newp", "-e"), 0)

	// A re-enables remote and pushes (its remove).
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "anchorA", "-u", "u", "--password=p"), 0)
	// B re-enables remote. Pre-pull merge (F1 L1 R0, L.mtime > F.mtime) → keep B's alpha.
	mustExit(t, runPswEnv(t, vaultB, envB, "add", "anchorB", "-u", "u", "--password=p"), 0)

	resB := runPswEnv(t, vaultB, envB, "get", "alpha", "--exact", "--stdout")
	mustExit(t, resB, 0)
	mustEqual(t, trimmed(resB), "newp")

	// A pulls and converges.
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "anchorA2", "-u", "u", "--password=p"), 0)
	resA := runPswEnv(t, vaultA, envA, "get", "alpha", "--exact", "--stdout")
	mustExit(t, resA, 0)
	mustEqual(t, trimmed(resA), "newp")
}

// TestSync_ChangeMain_CrossMergeBails: A changes main, B (stale) attempts mutation → red error, exit 1.
func TestSync_ChangeMain_CrossMergeBails(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "alpha", "-u", "u", "--password=p"), 0)

	vaultB, envB := addPeer(t, bare)
	// B fully synced now.

	// A changes main password and pushes.
	envAChange := withEnv(envA, "PSW_NEW_MAIN_PASSWORD", "newpass")
	mustExit(t, runPswEnv(t, vaultA, envAChange, "change", "main"), 0)

	// B (still on the old password) attempts a mutation. Pull brings in
	// A's change-main commit; decryption check fails → ErrForkUndecryptable.
	res := runPswEnv(t, vaultB, envB, "add", "beta", "-u", "u", "--password=p")
	mustExit(t, res, 1)
	mustContain(t, res.stdout, "Your main password was changed on another device")
}

// TestSync_NoRemoteConfigured: vault without remote=...; no warnings, no network attempts.
func TestSync_NoRemoteConfigured(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	// pswcfg without remote.
	if err := os.WriteFile(filepath.Join(dir, "pswcfg.toml"), []byte("clipboard_timeout = 20\n"), 0644); err != nil {
		t.Fatal(err)
	}
	env := gitTestEnv()
	res := runPswEnv(t, dir, env, "add", "foo", "-u", "u", "--password=p")
	mustExit(t, res, 0)
	if strings.Contains(res.stderr, "git push") || strings.Contains(res.stderr, "git fetch") {
		t.Fatalf("unexpected network attempt in stderr:\n%s", res.stderr)
	}
}

// TestSync_PSWGitRemoteZero: with remote configured but PSW_GIT_REMOTE=0, no network ops.
func TestSync_PSWGitRemoteZero(t *testing.T) {
	t.Parallel()
	vault, bare, env := newGitVaultWithRemote(t)
	beforeCount := bareCommitCount(t, bare)

	offlineEnv := withEnv(env, "PSW_GIT_REMOTE", "0")
	mustExit(t, runPswEnv(t, vault, offlineEnv, "add", "foo", "-u", "u", "--password=p"), 0)

	afterCount := bareCommitCount(t, bare)
	if afterCount != beforeCount {
		t.Fatalf("bare repo should not have advanced under PSW_GIT_REMOTE=0; before=%d after=%d", beforeCount, afterCount)
	}
}

// TestSync_SmartMerge_CaseInsensitiveDedup: two peers add the same name in
// different casings offline; after sync only one record survives.
func TestSync_SmartMerge_CaseInsensitiveDedup(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	vaultB, envB := addPeer(t, bare)

	offlineEnvA := withEnv(envA, "PSW_GIT_REMOTE", "0")
	offlineEnvB := withEnv(envB, "PSW_GIT_REMOTE", "0")

	mustExit(t, runPswEnv(t, vaultA, offlineEnvA, "add", "alice", "-u", "u", "--password=p"), 0)
	time.Sleep(mtimeSeparation)
	mustExit(t, runPswEnv(t, vaultB, offlineEnvB, "add", "ALICE", "-u", "u", "--password=p"), 0)

	mustExit(t, runPswEnv(t, vaultA, envA, "add", "anchorA", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vaultB, envB, "add", "anchorB", "-u", "u", "--password=p"), 0)

	// Dedup is unit-tested in internal/storage/merge_test.go; here we only
	// confirm the merged record resolves.
	resB := runPswEnv(t, vaultB, envB, "get", "alice", "--exact", "--stdout")
	mustExit(t, resB, 0)
	mustEqual(t, trimmed(resB), "p")
}

// TestSync_SmartMerge_WarningFormat: a remote-added record shows up in B's
// stderr with "Pulled" wording.
func TestSync_SmartMerge_WarningFormat(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	vaultB, envB := addPeer(t, bare)

	// A adds offline; B mutates locally; B's next online mutation pulls A's record.
	offlineEnvA := withEnv(envA, "PSW_GIT_REMOTE", "0")
	offlineEnvB := withEnv(envB, "PSW_GIT_REMOTE", "0")
	mustExit(t, runPswEnv(t, vaultA, offlineEnvA, "add", "alpha", "-u", "u", "--password=p"), 0)
	mustExit(t, runPswEnv(t, vaultB, offlineEnvB, "add", "beta", "-u", "u", "--password=p"), 0)

	mustExit(t, runPswEnv(t, vaultA, envA, "add", "anchorA", "-u", "u", "--password=p"), 0)
	res := runPswEnv(t, vaultB, envB, "add", "anchorB", "-u", "u", "--password=p")
	mustExit(t, res, 0)
	if !strings.Contains(res.stderr, "Pulled") {
		t.Fatalf("expected stderr to contain 'Pulled' merge warning:\n%s", res.stderr)
	}
	if !strings.Contains(res.stderr, "alpha") {
		t.Fatalf("expected stderr to mention pulled record name 'alpha':\n%s", res.stderr)
	}
}

// TestSync_SmartMerge_ByteEqualSilent: identical post-fork records on both
// sides → merge emits no warning.
func TestSync_SmartMerge_ByteEqualSilent(t *testing.T) {
	t.Parallel()
	vaultA, bare, envA := newGitVaultWithRemote(t)
	mustExit(t, runPswEnv(t, vaultA, envA, "add", "shared", "-u", "u", "--password=p"), 0)

	vaultB, envB := addPeer(t, bare)
	assertVaultHasRecords(t, vaultB, envB, "shared")

	// B mutates something else; "shared" is already byte-equal on both sides.
	res := runPswEnv(t, vaultB, envB, "add", "other", "-u", "u", "--password=p")
	mustExit(t, res, 0)
	if strings.Contains(res.stderr, "shared") {
		t.Fatalf("byte-equal record should not surface in merge warnings:\n%s", res.stderr)
	}
}

func withEnv(base map[string]string, k, v string) map[string]string {
	out := maps.Clone(base)
	out[k] = v
	return out
}
