package storage

import (
	"errors"
	"testing"
)

func TestHTTPSAuth_RejectsUserinfo(t *testing.T) {
	_, err := httpsAuth("https://user:pass@example.com/repo.git")
	if err == nil {
		t.Fatal("httpsAuth with userinfo returned nil error")
	}
	if !errors.Is(err, ErrRemoteCredentialsInURL) {
		t.Fatalf("err not ErrRemoteCredentialsInURL: %v", err)
	}
	if errors.Is(err, ErrShellGitNeeded) {
		t.Fatalf("err must not match ErrShellGitNeeded (would trigger silent shell-git fallback): %v", err)
	}
}

func TestHTTPSAuth_NoUserinfoNoHelperReturnsNil(t *testing.T) {
	// With no userinfo and no credential.helper configured we expect nil auth —
	// go-git will reach the server unauthenticated. shouldFallbackToShell logic
	// is exercised elsewhere.
	auth, err := httpsAuth("https://example.com/repo.git")
	if err != nil && errors.Is(err, ErrShellGitNeeded) {
		t.Skipf("credential.helper detected in this env; skip the no-auth assertion: %v", err)
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auth != nil {
		t.Fatalf("expected nil auth, got %T", auth)
	}
}
