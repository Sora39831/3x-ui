package crypto

import (
	"strings"
	"testing"
)

func TestHashPasswordAsBcrypt(t *testing.T) {
	hash, err := HashPasswordAsBcrypt("password123")
	if err != nil {
		t.Fatalf("HashPasswordAsBcrypt failed: %v", err)
	}
	if hash == "" {
		t.Fatal("hash should not be empty")
	}
	if hash == "password123" {
		t.Fatal("hash should not equal the plaintext password")
	}
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Fatalf("hash should have bcrypt prefix, got: %s", hash[:4])
	}
}

func TestHashAndCheckRoundTrip(t *testing.T) {
	passwords := []string{
		"password123",
		"",
		"very-long-password-with-special-chars-!@#$%^&*()",
		"unicode-密码-test",
	}
	for _, pw := range passwords {
		hash, err := HashPasswordAsBcrypt(pw)
		if err != nil {
			t.Fatalf("HashPasswordAsBcrypt(%q) failed: %v", pw, err)
		}
		if !CheckPasswordHash(hash, pw) {
			t.Errorf("CheckPasswordHash should return true for correct password %q", pw)
		}
	}
}

func TestCheckPasswordHashWrongPassword(t *testing.T) {
	hash, err := HashPasswordAsBcrypt("correct-password")
	if err != nil {
		t.Fatalf("HashPasswordAsBcrypt failed: %v", err)
	}
	if CheckPasswordHash(hash, "wrong-password") {
		t.Error("CheckPasswordHash should return false for wrong password")
	}
}

func TestCheckPasswordHashInvalidHash(t *testing.T) {
	if CheckPasswordHash("not-a-valid-hash", "password") {
		t.Error("CheckPasswordHash should return false for invalid hash")
	}
}

func TestDifferentPasswordsProduceDifferentHashes(t *testing.T) {
	hash1, _ := HashPasswordAsBcrypt("password1")
	hash2, _ := HashPasswordAsBcrypt("password2")
	if hash1 == hash2 {
		t.Error("different passwords should produce different hashes")
	}
}

func TestSamePasswordProducesDifferentHashes(t *testing.T) {
	hash1, _ := HashPasswordAsBcrypt("same-password")
	hash2, _ := HashPasswordAsBcrypt("same-password")
	if hash1 == hash2 {
		t.Error("bcrypt should use different salts, producing different hashes for same password")
	}
}
