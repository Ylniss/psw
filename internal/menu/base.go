package menu

import (
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/TwiN/go-color"
	"github.com/awnumar/memguard"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
)

// baseAction is embedded by every Action. Holds output/done/cancelled state
// and the input/yesNo/spinner sub-models managed by initX helpers.
type baseAction struct {
	output     []string
	transcript []string
	done       bool
	cancelled  bool

	// Terminal size, cached so a picker built after the async load can be sized.
	width, height int

	input   prompt.InputModel
	yesNo   prompt.YesNoModel
	spinner ui.SpinnerModel
}

func newBase(spinnerLabel string) baseAction {
	return baseAction{spinner: ui.NewSpinnerModel(spinnerLabel)}
}

func (b baseAction) Done() bool       { return b.done }
func (b baseAction) Cancelled() bool  { return b.cancelled }
func (b baseAction) Output() []string { return b.output }

// NewPassword is nil for every action but `change main`, which shadows this.
func (b baseAction) NewPassword() *memguard.Enclave { return nil }

// FooterHelp is empty for actions that render no help line; the others
// shadow this.
func (b baseAction) FooterHelp() string { return "" }

// finishErr appends a red error line and marks the action done.
func (b *baseAction) finishErr(err error) {
	b.output = append(b.output, color.InRed(err.Error()))
	b.done = true
}

// finish appends a pre-styled line and marks the action done.
func (b *baseAction) finish(line string) {
	b.output = append(b.output, line)
	b.done = true
}

func (b *baseAction) initInput(label string, password, animate bool) tea.Cmd {
	b.input = prompt.NewInputModel(label, password, animate)
	return b.input.Init()
}

// initYesNo returns nil; YesNoModel has no init cmd. Returned for symmetry
// with initInput / initSpinner.
func (b *baseAction) initYesNo(question string) tea.Cmd {
	b.yesNo = prompt.NewYesNoModel(question)
	return nil
}

// initYesNoWithHint is initYesNo with a hint line below the (y/n) prompt.
// Caller styles the hint.
func (b *baseAction) initYesNoWithHint(question, hint string) tea.Cmd {
	b.yesNo = prompt.NewYesNoModel(question).WithHint(hint)
	return nil
}

func (b *baseAction) initSpinner(label string) tea.Cmd {
	b.spinner = ui.NewSpinnerModel(label)
	return b.spinner.Init()
}

// captureSize records the terminal size. Called from every action's Update so
// a picker built later can be sized.
func (b *baseAction) captureSize(msg tea.Msg) {
	if w, ok := msg.(tea.WindowSizeMsg); ok {
		b.width, b.height = w.Width, w.Height
	}
}

// sizePicker forwards the cached terminal size to a freshly built picker;
// without it the list renders empty until the next resize.
func (b *baseAction) sizePicker(p *prompt.PickerModel) {
	if b.width <= 0 || b.height <= 0 {
		return
	}
	tuiutil.UpdateInPlace(p, tea.WindowSizeMsg{Width: b.width, Height: b.height})
}

// formatLoadError swaps fork-undecryptable for its user-facing banner.
func formatLoadError(err error) string {
	if errors.Is(err, storage.ErrForkUndecryptable) {
		return storage.ForkUndecryptableUserMessage
	}
	return err.Error()
}

// handleLoadingMsg classifies a loading-phase message. store is non-nil once
// storage is decrypted; until then the caller returns cmd — nil after an
// error, which is appended and marks the action done.
//
// password is the cached main password used to chain decryptCmd after pull.
func (b *baseAction) handleLoadingMsg(msg tea.Msg, password *memguard.Enclave) (store *storage.Storage, cmd tea.Cmd) {
	switch m := msg.(type) {
	case pullDoneMsg:
		if m.err != nil {
			b.finish(color.InRed(formatLoadError(m.err)))
			return nil, nil
		}
		b.transcript = append(b.transcript, m.warnings...)
		// Merge already decrypted; skip decryptCmd.
		if m.store != nil {
			return m.store, nil
		}
		return nil, tea.Batch(decryptCmd(password), b.initSpinner("Decrypting"))
	case storageLoadedMsg:
		if m.err != nil {
			b.finish(color.InRed(formatLoadError(m.err)))
			return nil, nil
		}
		return m.store, nil
	}
	return nil, tuiutil.UpdateInPlace(&b.spinner, msg)
}

// stepInput drives an InputModel sub-step. Cancel sets b.cancelled; done
// appends to transcript and returns the value.
func (b *baseAction) stepInput(msg tea.Msg) (value string, ready bool, cmd tea.Cmd) {
	cmd = tuiutil.UpdateInPlace(&b.input, msg)
	if b.input.Cancelled() {
		b.cancelled = true
		return "", false, nil
	}
	if b.input.Done() {
		b.transcript = append(b.transcript, formatInputLine(b.input))
		return b.input.Value(), true, nil
	}
	return "", false, cmd
}

// stepYesNo is stepInput for a YesNoModel.
func (b *baseAction) stepYesNo(msg tea.Msg) (answer bool, ready bool, cmd tea.Cmd) {
	cmd = tuiutil.UpdateInPlace(&b.yesNo, msg)
	if b.yesNo.Cancelled() {
		b.cancelled = true
		return false, false, nil
	}
	if b.yesNo.Done() {
		b.transcript = append(b.transcript, formatYesNoLine(b.yesNo))
		return b.yesNo.Answer(), true, nil
	}
	return false, false, cmd
}

// stepPicker is stepInput for a PickerModel. Transcript gets "> <selection>".
func (b *baseAction) stepPicker(p *prompt.PickerModel, msg tea.Msg) (selection string, ready bool, cmd tea.Cmd) {
	cmd = tuiutil.UpdateInPlace(p, msg)
	if p.Cancelled() {
		b.cancelled = true
		return "", false, nil
	}
	if p.Done() {
		sel := p.Selection()
		b.transcript = append(b.transcript, "> "+sel)
		return sel, true, nil
	}
	return "", false, cmd
}

// stepPickerMulti is stepPicker for multi-select. Returns ResolvedSelections;
// transcript gets one line per name.
func (b *baseAction) stepPickerMulti(p *prompt.PickerModel, msg tea.Msg) (selections []string, ready bool, cmd tea.Cmd) {
	cmd = tuiutil.UpdateInPlace(p, msg)
	if p.Cancelled() {
		b.cancelled = true
		return nil, false, nil
	}
	if p.Done() {
		sels := p.ResolvedSelections()
		for _, s := range sels {
			b.transcript = append(b.transcript, "> "+s)
		}
		return sels, true, nil
	}
	return nil, false, cmd
}

// prependBanner adds banner above v.Content and bumps the cursor row.
// Defensive cursor copy avoids aliasing the sub-model's cursor.
func prependBanner(v tea.View, banner string) tea.View {
	if banner == "" {
		return v
	}
	v.Content = banner + "\n" + v.Content
	if v.Cursor != nil {
		c := *v.Cursor
		c.Position.Y += strings.Count(banner, "\n") + 1
		v.Cursor = &c
	}
	return v
}

// prependTranscript prepends transcript lines above v.Content with a newline
// separator. Bumps the cursor row by the prepended row count.
func prependTranscript(v tea.View, transcript []string) tea.View {
	if len(transcript) == 0 {
		return v
	}
	return prependBanner(v, strings.Join(transcript, "\n"))
}

func formatYesNoLine(m prompt.YesNoModel) string {
	ans := "n"
	if m.Answer() {
		ans = "y"
	}
	return fmt.Sprintf("%s (y/n) %s", m.Question(), ans)
}

// formatInputLine renders a completed input as "Label: value"; hidden inputs
// (password / animated-stars) show asterisks matching the value's cell width.
func formatInputLine(m prompt.InputModel) string {
	val := m.Value()
	if m.Hidden() {
		val = strings.Repeat("*", lipgloss.Width(val))
	}
	return m.Prefix() + val
}
