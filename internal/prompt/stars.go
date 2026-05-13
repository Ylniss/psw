package prompt

import (
	"image/color"
	"math/rand/v2"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Star palette: ANSI 1..7 (skip "0" black).
var starPalette = []color.Color{
	lipgloss.Color("1"),
	lipgloss.Color("2"),
	lipgloss.Color("3"),
	lipgloss.Color("4"),
	lipgloss.Color("5"),
	lipgloss.Color("6"),
	lipgloss.Color("7"),
}

// styledStarByColor maps each palette entry to its pre-rendered "*" so
// View() doesn't re-allocate a lipgloss.Style per star per frame.
var styledStarByColor = func() map[color.Color]string {
	m := make(map[color.Color]string, len(starPalette))
	for _, c := range starPalette {
		m[c] = lipgloss.NewStyle().Foreground(c).Render("*")
	}
	return m
}()

// headerRestingColor is the menu's default header color; the flash palette
// excludes it so a flash always shifts to a different color.
var headerRestingColor color.Color = lipgloss.Color("6")

// headerFlashPalette is starPalette minus headerRestingColor — derived to
// keep the two in sync if starPalette changes.
var headerFlashPalette = paletteExcluding(starPalette, headerRestingColor)

func paletteExcluding(palette []color.Color, skip color.Color) []color.Color {
	out := make([]color.Color, 0, len(palette))
	for _, c := range palette {
		if c != skip {
			out = append(out, c)
		}
	}
	return out
}

const (
	starBlinkDuration = 500 * time.Millisecond
	starTickInterval  = 100 * time.Millisecond
)

type star struct {
	flashColor color.Color
	blinkUntil time.Time
}

type StarState struct {
	stars           []star
	rng             *rand.Rand
	lastStarIndex   int
	lastHeaderIndex int
}

func NewStarState() StarState {
	now := uint64(time.Now().UnixNano())
	return StarState{rng: rand.New(rand.NewPCG(now, now^0x9E3779B97F4A7C15))}
}

// pickDistinctIdx picks a uniformly random index in [0, paletteLen) that is
// not lastIdx. paletteLen must be >= 2; with 1 entry there's no choice. The
// zero-value lastIdx (= 0) just biases the very first pick away from index 0,
// which is negligible.
func (s *StarState) pickDistinctIdx(paletteLen, lastIdx int) int {
	if paletteLen <= 1 {
		return 0
	}
	s.ensureRNG()
	idx := s.rng.IntN(paletteLen - 1)
	if idx >= lastIdx {
		idx++
	}
	return idx
}

func (s *StarState) Len() int { return len(s.stars) }

func (s *StarState) Reset() { s.stars = s.stars[:0] }

func (s *StarState) Add(n int) {
	if n <= 0 {
		return
	}
	idx := s.pickDistinctIdx(len(starPalette), s.lastStarIndex)
	s.lastStarIndex = idx
	s.addWithColor(n, starPalette[idx])
}

// addWithColor appends n stars sharing one flashColor so they blink in unison.
func (s *StarState) addWithColor(n int, c color.Color) {
	if n <= 0 {
		return
	}
	until := time.Now().Add(starBlinkDuration)
	for range n {
		s.stars = append(s.stars, star{flashColor: c, blinkUntil: until})
	}
}

func (s *StarState) Remove(n int) {
	if n <= 0 {
		return
	}
	if n >= len(s.stars) {
		s.stars = s.stars[:0]
		return
	}
	s.stars = s.stars[:len(s.stars)-n]
}

func (s *StarState) Active() bool {
	now := time.Now()
	for _, st := range s.stars {
		if now.Before(st.blinkUntil) {
			return true
		}
	}
	return false
}

// View renders the star string. Stars within their blink window get foreground
// color; settled stars render as plain "*" (default terminal foreground).
func (s *StarState) View() string {
	if len(s.stars) == 0 {
		return ""
	}
	now := time.Now()
	var b strings.Builder
	for _, st := range s.stars {
		if now.Before(st.blinkUntil) {
			if styled, ok := styledStarByColor[st.flashColor]; ok {
				b.WriteString(styled)
				continue
			}
			b.WriteString(lipgloss.NewStyle().Foreground(st.flashColor).Render("*"))
			continue
		}
		b.WriteByte('*')
	}
	return b.String()
}

// ApplyKeystrokeAdd is called when the input grows by deltaChars. Per char,
// adds 1 or 2 stars (50/50). All stars added in this call share one randomly
// chosen palette color (never the same as the previous keystroke's color) so
// they blink in unison. Returns true if any stars were added.
func (s *StarState) ApplyKeystrokeAdd(deltaChars int) bool {
	if deltaChars <= 0 {
		return false
	}
	s.ensureRNG()
	idx := s.pickDistinctIdx(len(starPalette), s.lastStarIndex)
	s.lastStarIndex = idx
	c := starPalette[idx]
	total := 0
	for range deltaChars {
		total += 1 + s.rng.IntN(2) // 1 or 2
	}
	s.addWithColor(total, c)
	return true
}

// ApplyKeystrokeRemove is the deterministic delete path: removes |delta|
// stars (1 per backspace; more for ctrl-w). Returns true if state changed.
func (s *StarState) ApplyKeystrokeRemove(deltaChars int) bool {
	if deltaChars <= 0 || len(s.stars) == 0 {
		return false
	}
	s.Remove(deltaChars)
	return true
}

// ApplyEmpty resets visible stars when the input is fully cleared. Returns
// true if state changed.
func (s *StarState) ApplyEmpty() bool {
	if len(s.stars) == 0 {
		return false
	}
	s.Reset()
	return true
}

// RandomHeaderColor picks a color from the cyan-excluded header palette,
// never repeating the previous pick.
func (s *StarState) RandomHeaderColor() color.Color {
	idx := s.pickDistinctIdx(len(headerFlashPalette), s.lastHeaderIndex)
	s.lastHeaderIndex = idx
	return headerFlashPalette[idx]
}

func (s *StarState) ensureRNG() {
	if s.rng == nil {
		now := uint64(time.Now().UnixNano())
		s.rng = rand.New(rand.NewPCG(now, now^0x9E3779B97F4A7C15))
	}
}

type StarTickMsg time.Time

func StarTick() tea.Cmd {
	return tea.Tick(starTickInterval, func(t time.Time) tea.Msg {
		return StarTickMsg(t)
	})
}
