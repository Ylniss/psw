package menu

import (
	tea "charm.land/bubbletea/v2"
	"github.com/TwiN/go-color"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
	"github.com/ylniss/psw/internal/ui"
)

// baseAction is embedded by every concrete Action. It holds output/done/
// cancelled state plus the input/yesNo/spinner sub-models managed by the
// initX helpers below.
type baseAction struct {
	output     []string
	transcript []string
	done       bool
	cancelled  bool

	input   prompt.InputModel
	yesNo   prompt.YesNoModel
	spinner ui.SpinnerModel
}

// newBase returns a baseAction with the spinner pre-initialized.
func newBase(spinnerLabel string) baseAction {
	return baseAction{spinner: ui.NewSpinnerModel(spinnerLabel)}
}

func (b baseAction) Done() bool       { return b.done }
func (b baseAction) Cancelled() bool  { return b.cancelled }
func (b baseAction) Output() []string { return b.output }

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

// initYesNoWithHint is initYesNo plus a styled secondary line rendered below
// the (y/n) prompt. Caller owns the hint's coloring.
func (b *baseAction) initYesNoWithHint(question, hint string) tea.Cmd {
	b.yesNo = prompt.NewYesNoModel(question).WithHint(hint)
	return nil
}

func (b *baseAction) initSpinner(label string) tea.Cmd {
	b.spinner = ui.NewSpinnerModel(label)
	return b.spinner.Init()
}

// handleLoadingMsg classifies a loading-phase message for Add/Change/Get/
// Remove. Returns:
//
//	store != nil           — storage decrypted; caller advances to next phase.
//	store == nil, done     — error appended; baseAction.done is set; caller returns nil.
//	store == nil, cmd      — still waiting; caller returns the cmd.
//
// password is the cached main password used to chain decryptCmd after pull.
func (b *baseAction) handleLoadingMsg(msg tea.Msg, password string) (store *storage.Storage, done bool, cmd tea.Cmd) {
	switch m := msg.(type) {
	case pullDoneMsg:
		if m.err != nil {
			b.output = append(b.output, color.InRed(formatLoadError(m.err)))
			b.done = true
			return nil, true, nil
		}
		b.transcript = append(b.transcript, m.warnings...)
		b.spinner = ui.NewSpinnerModel("Decrypting")
		return nil, false, tea.Batch(decryptCmd(password), b.spinner.Init())
	case storageLoadedMsg:
		if m.err != nil {
			b.output = append(b.output, color.InRed(formatLoadError(m.err)))
			b.done = true
			return nil, true, nil
		}
		return m.store, false, nil
	}
	return nil, false, tuiutil.UpdateInPlace(&b.spinner, msg)
}

// stepInput drives an InputModel sub-step. Sets b.cancelled on cancel; on
// done appends to the transcript and returns the value.
func (b *baseAction) stepInput(input *prompt.InputModel, msg tea.Msg) (value string, ready bool, cmd tea.Cmd) {
	cmd = tuiutil.UpdateInPlace(input, msg)
	if input.Cancelled() {
		b.cancelled = true
		return "", false, nil
	}
	if input.Done() {
		b.transcript = append(b.transcript, formatInputLine(*input))
		return input.Value(), true, nil
	}
	return "", false, cmd
}

// stepYesNo is stepInput for a YesNoModel.
func (b *baseAction) stepYesNo(yn *prompt.YesNoModel, msg tea.Msg) (answer bool, ready bool, cmd tea.Cmd) {
	cmd = tuiutil.UpdateInPlace(yn, msg)
	if yn.Cancelled() {
		b.cancelled = true
		return false, false, nil
	}
	if yn.Done() {
		b.transcript = append(b.transcript, formatYesNoLine(*yn))
		return yn.Answer(), true, nil
	}
	return false, false, cmd
}

// stepPicker is the PickerModel counterpart of stepInput. Records the
// selection in the transcript with a "> " marker.
func (b *baseAction) stepPicker(p *storage.PickerModel, msg tea.Msg) (selection string, ready bool, cmd tea.Cmd) {
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

// stepPickerMulti drives a multi-mode PickerModel. Returns ResolvedSelections
// on done; appends one transcript line per returned name.
func (b *baseAction) stepPickerMulti(p *storage.PickerModel, msg tea.Msg) (selections []string, ready bool, cmd tea.Cmd) {
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
