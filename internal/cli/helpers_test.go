package cli

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
)

func TestHandleCmdErr(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantRet error
		wantOut string // "" means nothing may be printed
	}{
		{"prompt cancelled stops silently", prompt.ErrPromptCancelled, nil, ""},
		{"errSilentExit is an already-printed sentinel", errSilentExit, errSilentExit, ""},
		{"fork undecryptable prints the banner", storage.ErrForkUndecryptable, errSilentExit, "main password was changed on another device"},
		{"non-classified err prints and stops", errors.New("disk on fire"), nil, "disk on fire"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ret error
			out := captureStdout(t, func() { ret = handleCmdErr(tc.err) })
			if !errors.Is(ret, tc.wantRet) {
				t.Fatalf("ret = %v, want %v", ret, tc.wantRet)
			}
			if tc.wantOut == "" && out != "" {
				t.Fatalf("must not print, got %q", out)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Fatalf("output %q missing %q", out, tc.wantOut)
			}
		})
	}
}

func TestHandlePromptErr_NilDoesNotAbort(t *testing.T) {
	if handlePromptErr(nil) {
		t.Fatal("nil err must not abort")
	}
}

func TestHandlePromptErr_CancelledAbortsSilently(t *testing.T) {
	out := captureStdout(t, func() {
		if !handlePromptErr(prompt.ErrPromptCancelled) {
			t.Fatal("ErrPromptCancelled must abort")
		}
	})
	if out != "" {
		t.Fatalf("ErrPromptCancelled must not print, got %q", out)
	}
}

func TestHandlePromptErr_OtherPrintsAndAborts(t *testing.T) {
	out := captureStdout(t, func() {
		if !handlePromptErr(errors.New("input got eaten")) {
			t.Fatal("non-cancel err must abort")
		}
	})
	if !strings.Contains(out, "input got eaten") {
		t.Fatalf("expected err message in output, got %q", out)
	}
}

// captureStdout temporarily redirects os.Stdout for fn's duration and returns
// what was written. Restores the original Stdout via t.Cleanup so a panicking
// fn doesn't leave the test runner with a broken stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	fn()
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(out)
}
