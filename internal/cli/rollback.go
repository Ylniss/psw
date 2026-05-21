package cli

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/TwiN/go-color"
	"github.com/spf13/cobra"

	"github.com/ylniss/psw/internal/prompt"
	"github.com/ylniss/psw/internal/storage"
)

const (
	envRollbackTarget = "PSW_ROLLBACK_TARGET"
	envRollbackYes    = "PSW_ROLLBACK_YES"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back records to a past commit's snapshot",
	Long: `Pick a past commit; rollback creates a new commit replacing the
current records with that snapshot. History is preserved.

Rolling back across a 'change main' boundary is refused — the target commit
would be encrypted under a different main password.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ok, err := storage.IsGitRepo()
		if err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}
		if !ok {
			fmt.Println(color.InRed("Nothing to roll back to — storage isn't tracked by git."))
			return errSilentExit
		}

		store, err := storage.GetOrCreateForMutate()
		if done, ret := handleCmdErr(err); done {
			return ret
		}

		entries, err := storage.GitLog()
		if done, ret := handleCmdErr(err); done {
			return ret
		}
		headShort, err := storage.HeadShortSHA()
		if err != nil {
			fmt.Println(color.InRed(err.Error()))
			return errSilentExit
		}

		picks := make([]storage.LogEntry, 0, len(entries))
		for _, e := range entries {
			if e.ShortSHA != headShort {
				picks = append(picks, e)
			}
		}
		if len(picks) == 0 {
			fmt.Println("Nothing to roll back to.")
			return nil
		}
		slices.Reverse(picks) // newest first — most likely rollback targets at the top

		target, err := chooseRollbackTarget(picks)
		if err != nil {
			if errors.Is(err, storage.ErrPickerCancelled) {
				return nil
			}
			if done, ret := handleCmdErr(err); done {
				return ret
			}
		}
		if target == nil {
			return nil
		}

		records, err := storage.LoadCommitRecords(target.ShortSHA, store.MainPassword)
		if errors.Is(err, storage.ErrForkUndecryptable) {
			fmt.Println(color.InRed("Can't roll back to this snapshot — it was encrypted with a different main password"))
			return errSilentExit
		}
		if done, ret := handleCmdErr(err); done {
			return ret
		}

		if !confirmRollback(*target) {
			return nil
		}

		if done, ret := handleCmdErr(storage.ApplyRollback(store, *target, records)); done {
			return ret
		}
		fmt.Printf("Rolled back to %s: %s\n", color.InCyan(target.ShortSHA), target.Message)
		return nil
	},
}

// chooseRollbackTarget returns the selected LogEntry, or nil if the user
// cancelled. Uses PSW_ROLLBACK_TARGET (matched against ShortSHA prefix) when
// set; otherwise opens the interactive picker. Returns an error when env-var
// bypass mode is partial or the target SHA isn't in picks.
func chooseRollbackTarget(picks []storage.LogEntry) (*storage.LogEntry, error) {
	envTarget := os.Getenv(envRollbackTarget)
	envYes := os.Getenv(envRollbackYes)
	if envTarget != "" || envYes != "" {
		if envTarget == "" || envYes == "" {
			fmt.Println(color.InRed(envRollbackTarget + " and " + envRollbackYes + " must be set together"))
			return nil, errSilentExit
		}
		for i := range picks {
			if picks[i].ShortSHA == envTarget {
				return &picks[i], nil
			}
		}
		fmt.Printf("%s: no commit with short-sha %s\n", color.InRed("rollback"), envTarget)
		return nil, errSilentExit
	}

	if !prompt.IsTTY() {
		fmt.Println(color.InRed("rollback requires an interactive terminal"))
		return nil, errSilentExit
	}

	byLabel := make(map[string]storage.LogEntry, len(picks))
	labels := make([]string, len(picks))
	for i, e := range picks {
		label := fmt.Sprintf("%s  %s  %s",
			e.ShortSHA,
			e.Time.Format("2006-01-02 15:04"),
			e.Message,
		)
		labels[i] = label
		byLabel[label] = e
	}

	chosen, err := storage.GetRecordNameInteractive(labels, nil)
	if err != nil {
		return nil, err
	}
	entry, ok := byLabel[chosen]
	if !ok {
		return nil, fmt.Errorf("internal: picker returned unrecognized label %q", chosen)
	}
	return &entry, nil
}

func confirmRollback(target storage.LogEntry) bool {
	if os.Getenv(envRollbackYes) != "" {
		return true
	}
	return prompt.YesOrNo(fmt.Sprintf("Replace records with snapshot from %s (%s)?", target.ShortSHA, target.Message))
}
