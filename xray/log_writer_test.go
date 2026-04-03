package xray

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/op/go-logging"
)

func init() {
	// Initialize logger for tests that use LogWriter (which calls logger.Debug/Error/etc.)
	logger.InitLogger(logging.DEBUG)
}

func TestNewLogWriter(t *testing.T) {
	lw := NewLogWriter()
	if lw == nil {
		t.Fatal("NewLogWriter should not return nil")
	}
}

func TestLogWriter_Write_CrashDetection(t *testing.T) {
	lw := NewLogWriter()
	msg := []byte("panic: runtime error: index out of range")
	n, err := lw.Write(msg)
	if err != nil {
		t.Fatalf("Write should not return error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned %d, expected %d", n, len(msg))
	}
}

func TestLogWriter_Write_FatalError(t *testing.T) {
	lw := NewLogWriter()
	msg := []byte("fatal error: concurrent map writes")
	n, err := lw.Write(msg)
	if err != nil {
		t.Fatalf("Write should not return error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned %d, expected %d", n, len(msg))
	}
}

func TestLogWriter_Write_Exception(t *testing.T) {
	lw := NewLogWriter()
	msg := []byte("unhandled exception occurred")
	n, err := lw.Write(msg)
	if err != nil {
		t.Fatalf("Write should not return error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned %d, expected %d", n, len(msg))
	}
}

func TestLogWriter_Write_EmptyMessage(t *testing.T) {
	lw := NewLogWriter()
	n, err := lw.Write([]byte(""))
	if err != nil {
		t.Fatalf("Write should not error: %v", err)
	}
	if n != 0 {
		t.Errorf("Write returned %d, expected 0", n)
	}
}

func TestLogWriter_Write_TLSErrorSuppressed(t *testing.T) {
	lw := NewLogWriter()
	msg := []byte("some tls handshake error occurred")
	n, err := lw.Write(msg)
	if err != nil {
		t.Fatalf("Write should not return error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned %d, expected %d", n, len(msg))
	}
}

func TestLogWriter_Write_FailedKeyword(t *testing.T) {
	lw := NewLogWriter()
	msg := []byte("connection failed to remote")
	n, err := lw.Write(msg)
	if err != nil {
		t.Fatalf("Write should not return error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write returned %d, expected %d", n, len(msg))
	}
}
