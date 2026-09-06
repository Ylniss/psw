package prompt

import (
	"math"
	"testing"
)

func TestStarState_AddRemoveLen(t *testing.T) {
	s := NewStarState()
	if s.Len() != 0 {
		t.Fatalf("zero len, got %d", s.Len())
	}
	s.Add(3)
	if s.Len() != 3 {
		t.Fatalf("after Add(3) want 3, got %d", s.Len())
	}
	s.Remove(1)
	if s.Len() != 2 {
		t.Fatalf("after Remove(1) want 2, got %d", s.Len())
	}
	s.Remove(99) // clamp
	if s.Len() != 0 {
		t.Fatalf("over-remove should empty, got %d", s.Len())
	}
}

func TestStarState_Reset(t *testing.T) {
	s := NewStarState()
	s.Add(5)
	s.Reset()
	if s.Len() != 0 {
		t.Fatalf("after Reset want 0, got %d", s.Len())
	}
}

func TestStarState_ActiveAfterAdd(t *testing.T) {
	s := NewStarState()
	s.Add(1)
	if !s.Active() {
		t.Fatalf("Active should be true after Add (within blink window)")
	}
}

func TestStarState_ZeroValueUsable(t *testing.T) {
	var s StarState
	s.Add(1)
	if s.Len() != 1 {
		t.Fatalf("zero-value should accept Add, got len %d", s.Len())
	}
}

func TestStarState_ApplyKeystrokeRemove(t *testing.T) {
	s := NewStarState()
	s.Add(5)
	if !s.ApplyKeystrokeRemove(2) {
		t.Fatalf("remove of 2 should change state")
	}
	if s.Len() != 3 {
		t.Fatalf("want 3 after remove(2), got %d", s.Len())
	}
	if s.ApplyKeystrokeRemove(0) {
		t.Fatalf("remove of 0 should not change state")
	}
}

func TestStarState_ApplyEmpty(t *testing.T) {
	s := NewStarState()
	s.Add(3)
	if !s.ApplyEmpty() {
		t.Fatalf("empty on non-empty should change state")
	}
	if s.Len() != 0 {
		t.Fatalf("after ApplyEmpty want 0, got %d", s.Len())
	}
	if s.ApplyEmpty() {
		t.Fatalf("empty on empty should not change state")
	}
}

// Distribution check: per-char, ApplyKeystrokeAdd rolls 1 or 2 (50/50), so
// over many trials the mean delta should be ~1.5.
func TestStarState_ApplyKeystrokeAdd_Distribution(t *testing.T) {
	const trials = 5000
	s := NewStarState()
	for range trials {
		s.ApplyKeystrokeAdd(1)
	}
	mean := float64(s.Len()) / float64(trials)
	if math.Abs(mean-1.5) > 0.1 {
		t.Fatalf("expected mean delta ~+1.5, got %.3f over %d trials", mean, trials)
	}
}

// All stars added in one ApplyKeystrokeAdd call must share one blinkColor.
func TestStarState_ApplyKeystrokeAdd_SingleColor(t *testing.T) {
	s := NewStarState()
	for trial := range 50 {
		s.Reset()
		s.ApplyKeystrokeAdd(5) // 5-10 stars added with one color
		if s.Len() < 5 {
			t.Fatalf("trial %d: too few stars (%d)", trial, s.Len())
		}
		first := s.stars[0].blinkColor
		for i, st := range s.stars {
			if st.blinkColor != first {
				t.Fatalf("trial %d: star %d color %v differs from first %v", trial, i, st.blinkColor, first)
			}
		}
	}
}

func TestStarState_ApplyKeystrokeAdd_NoChangeOnZero(t *testing.T) {
	s := NewStarState()
	if s.ApplyKeystrokeAdd(0) {
		t.Fatalf("ApplyKeystrokeAdd(0) should not change state")
	}
}

func TestStarState_RandomHeaderColorNotNil(t *testing.T) {
	s := NewStarState()
	c := s.RandomHeaderColor()
	if c == nil {
		t.Fatalf("RandomHeaderColor returned nil")
	}
}

func TestStarState_ApplyKeystrokeAdd_NeverRepeatsColor(t *testing.T) {
	s := NewStarState()
	s.ApplyKeystrokeAdd(1)
	prev := s.stars[len(s.stars)-1].blinkColor
	for i := range 200 {
		s.Reset()
		s.ApplyKeystrokeAdd(1)
		cur := s.stars[len(s.stars)-1].blinkColor
		if cur == prev {
			t.Fatalf("iteration %d: color repeated (%v)", i, cur)
		}
		prev = cur
	}
}

func TestStarState_RandomHeaderColor_NeverRepeats(t *testing.T) {
	s := NewStarState()
	prev := s.RandomHeaderColor()
	for i := range 200 {
		cur := s.RandomHeaderColor()
		if cur == prev {
			t.Fatalf("iteration %d: header color repeated (%v)", i, cur)
		}
		prev = cur
	}
}
