package app

import "unicode/utf8"

// ErrorText bounds displayed diagnostics, including echoed user input. Keep the
// original error for exit codes and errors.Is/errors.As.
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) <= 200 {
		return s
	}
	end := 197
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end] + "..."
}

type displayError struct{ error }

func (e displayError) Error() string { return ErrorText(e.error) }
func (e displayError) Unwrap() error { return e.error }
func BoundedError(err error) error {
	if err == nil {
		return nil
	}
	return displayError{err}
}
