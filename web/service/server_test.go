package service

import (
	"testing"
)

func TestIsValidGeofileName_Valid(t *testing.T) {
	svc := &ServerService{}
	valid := []string{
		"geoip.dat",
		"geosite.dat",
		"custom-file_v2.dat",
	}
	for _, name := range valid {
		if !svc.IsValidGeofileName(name) {
			t.Errorf("IsValidGeofileName(%q) should return true", name)
		}
	}
}

func TestIsValidGeofileName_PathTraversal(t *testing.T) {
	svc := &ServerService{}
	invalid := []string{
		"../geoip.dat",
		"../../etc/passwd",
		"subdir/geoip.dat",
		"geoip.dat/../../../etc",
		"..\\geoip.dat",
	}
	for _, name := range invalid {
		if svc.IsValidGeofileName(name) {
			t.Errorf("IsValidGeofileName(%q) should return false (path traversal)", name)
		}
	}
}

func TestIsValidGeofileName_Empty(t *testing.T) {
	svc := &ServerService{}
	if svc.IsValidGeofileName("") {
		t.Error("IsValidGeofileName(\"\") should return false")
	}
}

func TestIsValidGeofileName_NoDatExtension(t *testing.T) {
	svc := &ServerService{}
	invalid := []string{
		"geoip.txt",
		"geosite",
		"file.exe",
		"script.sh",
	}
	for _, name := range invalid {
		if svc.IsValidGeofileName(name) {
			t.Errorf("IsValidGeofileName(%q) should return false (no .dat extension)", name)
		}
	}
}

func TestIsValidGeofileName_SpecialChars(t *testing.T) {
	svc := &ServerService{}
	invalid := []string{
		"geoip$.dat",
		"geoip!.dat",
		"geoip;.dat",
		"geoip .dat",
		"geoip@attack.dat",
	}
	for _, name := range invalid {
		if svc.IsValidGeofileName(name) {
			t.Errorf("IsValidGeofileName(%q) should return false (special chars)", name)
		}
	}
}

func TestLogEntryContains(t *testing.T) {
	tests := []struct {
		line     string
		suffixes []string
		want     bool
	}{
		// The implementation checks strings.Contains(line, sfx+"]")
		{"line with freedom]", []string{"freedom"}, true},
		{"line with blackhole]", []string{"blackhole"}, true},
		{"freedom outbound", []string{"freedom"}, false},
		{"blackhole outbound", []string{"blackhole"}, false},
		{"freedom outbound", []string{"blackhole"}, false},
		{"some log line", []string{}, false},
		{"line with freedom] and blackhole]", []string{"freedom", "blackhole"}, true},
		{"line with freedom] and blackhole]", []string{"other"}, false},
	}
	for _, tt := range tests {
		got := logEntryContains(tt.line, tt.suffixes)
		if got != tt.want {
			t.Errorf("logEntryContains(%q, %v) = %v, want %v", tt.line, tt.suffixes, got, tt.want)
		}
	}
}
