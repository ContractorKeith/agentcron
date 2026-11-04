package app

import (
	"errors"
	"github.com/ContractorKeith/agentcron/internal/config"
)

type ExitCode int

const (
	OK ExitCode = iota
	Runtime
	Usage
	NotInitialized
	Locked
	Refused
	RunUnsuccessful
)

type Error struct {
	Code ExitCode
	Err  error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }
func Code(err error) ExitCode {
	if err == nil {
		return OK
	}
	var invalid *config.ValidationError
	if errors.As(err, &invalid) {
		return Usage
	}
	var coded *Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	return Runtime
}
