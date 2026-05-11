// Package tuiutil holds generic helpers shared across embeddable tea.Model
// primitives.
package tuiutil

import tea "charm.land/bubbletea/v2"

// Finishable signals completion via Done() or cancellation via Cancelled().
type Finishable interface {
	tea.Model
	Done() bool
	Cancelled() bool
}

// QuittingWrapper wraps a Finishable so a top-level tea.NewProgram exits when the
// inner model is done or cancelled.
type QuittingWrapper[M Finishable] struct{ M M }

func (q QuittingWrapper[M]) Init() tea.Cmd { return q.M.Init() }

func (q QuittingWrapper[M]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	raw, cmd := q.M.Update(msg)
	q.M = raw.(M)
	if q.M.Done() || q.M.Cancelled() {
		return q, tea.Batch(cmd, tea.Quit)
	}
	return q, cmd
}

func (q QuittingWrapper[M]) View() tea.View { return q.M.View() }

// UpdateInPlace forwards msg to *dst, replacing dst with the updated value.
// Returns the model's Cmd.
func UpdateInPlace[M tea.Model](dst *M, msg tea.Msg) tea.Cmd {
	raw, cmd := (*dst).Update(msg)
	*dst = raw.(M)
	return cmd
}
