package menu

import (
	tea "charm.land/bubbletea/v2"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
	"github.com/ylniss/psw/internal/tuiutil"
)

// baseAction holds the fields and accessors shared by every menu Action.
// Each concrete action embeds it; promoted methods satisfy the common
// portion of the Action interface.
type baseAction struct {
	output     []string
	transcript []string
	done       bool
	cancelled  bool
}

func (b baseAction) Done() bool       { return b.done }
func (b baseAction) Cancelled() bool  { return b.cancelled }
func (b baseAction) Output() []string { return b.output }

// stepInput drives a sub-model InputModel. On cancel: sets b.cancelled and
// returns ready=false. On done: appends the input's line to the transcript
// and returns the value with ready=true. Otherwise returns the input's cmd.
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

// stepYesNo is the YesNoModel counterpart of stepInput.
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
