package cli

import (
	"errors"
	"fmt"
	"os"

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
		if err != nil {
			return handleCmdErr(err)
		}

		labels, byLabel, err := storage.RollbackPicks()
		if err != nil {
			return handleCmdErr(err)
		}
		if len(labels) == 0 {
			fmt.Println("Nothing to roll back to.")
			return nil
		}

		target, err := chooseRollbackTarget(labels, byLabel)
		if errors.Is(err, prompt.ErrPickerCancelled) {
			return nil
		}
		if err != nil {
			return handleCmdErr(err)
		}

		records, err := storage.LoadCommitRecords(target.ShortSHA, store.MainPassword)
		if errors.Is(err, storage.ErrForkUndecryptable) {
			fmt.Println(color.InRed("Can't roll back to this snapshot — it was encrypted with a different main password"))
			return errSilentExit
		}
		if err != nil {
			return handleCmdErr(err)
		}

		if !confirmRollback(*target) {
			return nil
		}

		if err := storage.ApplyRollback(store, *target, records); err != nil {
			return handleCmdErr(err)
		}
		fmt.Printf("Rolled back to %s: %s\n", color.InCyan(target.ShortSHA), target.Message)
		return nil
	},
}

// chooseRollbackTarget returns the selected LogEntry, or nil if the user
// cancelled. Uses PSW_ROLLBACK_TARGET (matched against ShortSHA prefix) when
// set; otherwise opens the interactive picker. Returns an error when env-var
// bypass mode is partial or the target SHA isn't among the candidates.
func chooseRollbackTarget(labels []string, byLabel map[string]storage.LogEntry) (*storage.LogEntry, error) {
	envTarget := os.Getenv(envRollbackTarget)
	envYes := os.Getenv(envRollbackYes)
	if envTarget != "" || envYes != "" {
		if envTarget == "" || envYes == "" {
			fmt.Println(color.InRed(envRollbackTarget + " and " + envRollbackYes + " must be set together"))
			return nil, errSilentExit
		}
		for _, e := range byLabel {
			if e.ShortSHA == envTarget {
				return &e, nil
			}
		}
		fmt.Printf("%s: no commit with short-sha %s\n", color.InRed("rollback"), envTarget)
		return nil, errSilentExit
	}

	if !prompt.IsTTY() {
		fmt.Println(color.InRed("rollback requires an interactive terminal"))
		return nil, errSilentExit
	}

	chosen, err := prompt.GetRecordNameInteractive(labels, nil)
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
