package storage

import "github.com/ylniss/psw/internal/passgen"

// GenerateRecordPassword applies [password_gen] config (with defaults for
// missing keys) and generates a password. Invalid config emits a yellow
// warning via WarnSink and falls back to defaults — never blocks the action.
func GenerateRecordPassword() (string, error) {
	opts := AppConfig.PasswordGen.Resolve()
	if err := opts.Validate(); err != nil {
		Warn("password_gen invalid: %v. Falling back to defaults.", err)
		opts = passgen.DefaultOptions()
	}
	return passgen.Generate(opts)
}
