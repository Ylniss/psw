package tests

// These tests share menulayout package state. No t.Parallel() — the parallel
// integration tests in this dir never touch menulayout.

import (
	"strings"
	"testing"

	"github.com/ylniss/psw/internal/menulayout"
)

func TestMenulayout_RenderPassThroughWhenInactive(t *testing.T) {
	menulayout.Clear()
	got := menulayout.Render("hello\n")
	if got != "hello\n" {
		t.Fatalf("Render(inactive) = %q, want %q", got, "hello\n")
	}
}

func TestMenulayout_RenderAppliesIndent(t *testing.T) {
	menulayout.Set(4, 0)
	defer menulayout.Clear()
	got := menulayout.Render("hello")
	if !strings.HasPrefix(got, "    ") {
		t.Fatalf("Render did not indent: %q", got)
	}
	if !strings.Contains(got, "hello") {
		t.Fatalf("Render lost content: %q", got)
	}
}

func TestMenulayout_RenderPreservesTrailingNewline(t *testing.T) {
	menulayout.Set(2, 0)
	defer menulayout.Clear()
	if got := menulayout.Render("hello\n"); !strings.HasSuffix(got, "\n") {
		t.Fatalf("trailing newline lost: %q", got)
	}
	if got := menulayout.Render("hello"); strings.HasSuffix(got, "\n") {
		t.Fatalf("unwanted trailing newline: %q", got)
	}
}

func TestMenulayout_RenderWrapsToWidth(t *testing.T) {
	menulayout.Set(0, 10)
	defer menulayout.Clear()
	got := menulayout.Render("hello world foo bar")
	if !strings.Contains(got, "\n") {
		t.Fatalf("expected wrap to insert newline, got %q", got)
	}
}

func TestMenulayout_RenderIndentPassThroughWhenInactive(t *testing.T) {
	menulayout.Clear()
	if got := menulayout.RenderIndent("hi"); got != "hi" {
		t.Fatalf("RenderIndent(inactive) = %q, want %q", got, "hi")
	}
}

func TestMenulayout_RenderIndentAppliesIndentNoWrap(t *testing.T) {
	menulayout.Set(3, 5)
	defer menulayout.Clear()
	got := menulayout.RenderIndent("hello world foo bar")
	if strings.Contains(got, "\n") {
		t.Fatalf("RenderIndent wrapped unexpectedly: %q", got)
	}
	if !strings.HasPrefix(got, "   ") {
		t.Fatalf("RenderIndent did not indent: %q", got)
	}
}

func TestMenulayout_IndentGetter(t *testing.T) {
	menulayout.Set(7, 99)
	defer menulayout.Clear()
	if got := menulayout.Indent(); got != 7 {
		t.Fatalf("Indent() = %d, want 7", got)
	}
}
