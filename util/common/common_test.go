package common

import (
	"errors"
	"strings"
	"testing"
)

func TestNewErrorf(t *testing.T) {
	err := NewErrorf("invalid port: %d", 8080)
	if err == nil {
		t.Fatal("NewErrorf should return non-nil error")
	}
	expected := "invalid port: 8080"
	if err.Error() != expected {
		t.Errorf("NewErrorf returned %q, expected %q", err.Error(), expected)
	}
}

func TestNewError(t *testing.T) {
	err := NewError("something", " went wrong")
	if err == nil {
		t.Fatal("NewError should return non-nil error")
	}
	if !strings.Contains(err.Error(), "something") {
		t.Errorf("NewError should contain 'something', got %q", err.Error())
	}
}

func TestRecoverWithoutPanic(t *testing.T) {
	recovered := Recover("")
	if recovered != nil {
		t.Errorf("Recover should return nil when no panic occurred, got %v", recovered)
	}
}

func TestFormatTrafficBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0.00B"},
		{1, "1.00B"},
		{512, "512.00B"},
		{1023, "1023.00B"},
	}
	for _, tt := range tests {
		result := FormatTraffic(tt.input)
		if result != tt.expected {
			t.Errorf("FormatTraffic(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatTrafficKB(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{1024, "1.00KB"},
		{1536, "1.50KB"},
		{2048, "2.00KB"},
	}
	for _, tt := range tests {
		result := FormatTraffic(tt.input)
		if result != tt.expected {
			t.Errorf("FormatTraffic(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatTrafficMB(t *testing.T) {
	result := FormatTraffic(1048576) // 1 MB
	expected := "1.00MB"
	if result != expected {
		t.Errorf("FormatTraffic(1048576) = %q, want %q", result, expected)
	}
}

func TestFormatTrafficGB(t *testing.T) {
	result := FormatTraffic(1073741824) // 1 GB
	expected := "1.00GB"
	if result != expected {
		t.Errorf("FormatTraffic(1073741824) = %q, want %q", result, expected)
	}
}

func TestFormatTrafficTB(t *testing.T) {
	result := FormatTraffic(1099511627776) // 1 TB
	expected := "1.00TB"
	if result != expected {
		t.Errorf("FormatTraffic(1099511627776) = %q, want %q", result, expected)
	}
}

func TestFormatTrafficPB(t *testing.T) {
	result := FormatTraffic(1125899906842624) // 1 PB
	expected := "1.00PB"
	if result != expected {
		t.Errorf("FormatTraffic(1125899906842624) = %q, want %q", result, expected)
	}
}

func TestFormatTrafficLargePB(t *testing.T) {
	// Value exceeding PB should stay in PB
	result := FormatTraffic(11258999068426240) // 10 PB
	if !strings.HasSuffix(result, "PB") {
		t.Errorf("FormatTraffic should cap at PB, got %q", result)
	}
}

func TestCombineAllNil(t *testing.T) {
	err := Combine(nil, nil, nil)
	if err != nil {
		t.Errorf("Combine(nil, nil, nil) should return nil, got %v", err)
	}
}

func TestCombineNoArgs(t *testing.T) {
	err := Combine()
	if err != nil {
		t.Errorf("Combine() should return nil, got %v", err)
	}
}

func TestCombineSingleError(t *testing.T) {
	input := errors.New("test error")
	err := Combine(input)
	if err == nil {
		t.Fatal("Combine should return non-nil when an error is present")
	}
	if !strings.Contains(err.Error(), "test error") {
		t.Errorf("Combine should contain 'test error', got %q", err.Error())
	}
}

func TestCombineMultipleErrors(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	combined := Combine(err1, nil, err2)
	if combined == nil {
		t.Fatal("Combine should return non-nil when errors are present")
	}
	s := combined.Error()
	if !strings.Contains(s, "error 1") {
		t.Errorf("Combined error should contain 'error 1', got %q", s)
	}
	if !strings.Contains(s, "error 2") {
		t.Errorf("Combined error should contain 'error 2', got %q", s)
	}
}

func TestCombineFiltersNils(t *testing.T) {
	err1 := errors.New("real error")
	combined := Combine(nil, err1, nil)
	if combined == nil {
		t.Fatal("Combine should return non-nil when at least one error is present")
	}
	if !strings.Contains(combined.Error(), "real error") {
		t.Errorf("Combined error should contain 'real error', got %q", combined.Error())
	}
}
