package cmd

// exitError carries a process exit code alongside the underlying error, so
// Execute can translate it into os.Exit(code) while RunE functions keep
// returning ordinary errors (which is what makes them testable: tests call
// rootCmd.Execute() directly and inspect the returned error instead of the
// process exit code).
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// newExitError wraps err so Execute exits with code. Returns nil if err is nil.
func newExitError(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}
