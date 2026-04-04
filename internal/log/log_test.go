package log

import (
	"errors"
	"testing"
)

func TestSageError(t *testing.T) {
	err := E("db.Open", errors.New("connection refused"))
	if err.Error() != "db.Open: connection refused" {
		t.Errorf("unexpected error string: %s", err.Error())
	}

	err2 := EP("config.Load", "/path/to/config.yaml", errors.New("not found"))
	if err2.Error() != "config.Load [/path/to/config.yaml]: not found" {
		t.Errorf("unexpected error string: %s", err2.Error())
	}

	// Unwrap
	inner := errors.New("inner")
	wrapped := E("op", inner)
	if !errors.Is(wrapped, inner) {
		t.Error("Unwrap should return inner error")
	}
}

func TestSetVerbose(t *testing.T) {
	// Should not panic
	SetVerbose(true)
	Debug("test debug message")
	SetVerbose(false)
	Info("test info message")
}
