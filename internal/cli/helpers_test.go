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

func TestReportCmdErr_NilProceeds(t *testing.T) {
	done, ret := handleCmdErr(nil)
	if done {
		t.Fatal("nil err should not stop the caller")
	}
	if ret != nil {
		t.Fatalf("nil err should return nil, got %v", ret)
	}
}

func TestReportCmdErr_PromptCancelledExitsSilently(t *testing.T) {
	out := captureStdout(t, func() {
		done, ret := handleCmdErr(prompt.ErrPromptCancelled)
		if !done {
			t.Fatal("ErrPromptCancelled should stop the caller")
		}
		if ret != nil {
			t.Fatalf("ErrPromptCancelled should return nil ret, got %v", ret)
		}
	})
	if out != "" {
		t.Fatalf("ErrPromptCancelled must not print, got %q", out)
	}
}

func TestReportCmdErr_ErrExitPropagates(t *testing.T) {
	out := captureStdout(t, func() {
		done, ret := handleCmdErr(errSilentExit)
		if !done {
			t.Fatal("errSilentExit should stop the caller")
		}
		if !errors.Is(ret, errSilentExit) {
			t.Fatalf("errSilentExit should propagate, got %v", ret)
		}
	})
	if out != "" {
		t.Fatalf("errSilentExit is already-printed sentinel; must not print, got %q", out)
	}
}

func TestReportCmdErr_ForkUndecryptablePrintsAndExits(t *testing.T) {
	out := captureStdout(t, func() {
		done, ret := handleCmdErr(storage.ErrForkUndecryptable)
		if !done {
			t.Fatal("ErrForkUndecryptable should stop the caller")
		}
		if !errors.Is(ret, errSilentExit) {
			t.Fatalf("ErrForkUndecryptable should return errSilentExit, got %v", ret)
		}
	})
	if !strings.Contains(out, "re-encrypted") {
		t.Fatalf("expected fork banner in output, got %q", out)
	}
}

func TestReportCmdErr_OtherPrintsAndContinues(t *testing.T) {
	out := captureStdout(t, func() {
		done, ret := handleCmdErr(errors.New("disk on fire"))
		if !done {
			t.Fatal("non-classified err should stop the caller (soft fail)")
		}
		if ret != nil {
			t.Fatalf("non-classified err should return nil ret, got %v", ret)
		}
	})
	if !strings.Contains(out, "disk on fire") {
		t.Fatalf("expected err message in output, got %q", out)
	}
}

func TestReportPromptErr_NilDoesNotAbort(t *testing.T) {
	if handlePromptErr(nil) {
		t.Fatal("nil err must not abort")
	}
}

func TestReportPromptErr_CancelledAbortsSilently(t *testing.T) {
	out := captureStdout(t, func() {
		if !handlePromptErr(prompt.ErrPromptCancelled) {
			t.Fatal("ErrPromptCancelled must abort")
		}
	})
	if out != "" {
		t.Fatalf("ErrPromptCancelled must not print, got %q", out)
	}
}

func TestReportPromptErr_OtherPrintsAndAborts(t *testing.T) {
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
