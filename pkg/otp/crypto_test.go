package otp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func testConfig() Config {
	return Config{Enabled: true, AdminToken: strings.Repeat("a", 40), HMACKey: bytes.Repeat([]byte{1}, 32), EncryptionKey: bytes.Repeat([]byte{2}, 32), AllowSandbox: true, RetentionDays: 30}
}
func TestCodeAndEncryption(t *testing.T) {
	s := NewService(nil, testConfig(), nil)
	for i := 0; i < 100; i++ {
		if !codePattern.MatchString(newCode()) {
			t.Fatal("invalid code")
		}
	}
	encrypted, err := s.seal("123456", "challenge-a")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.open(encrypted, "challenge-a")
	if err != nil || plain != "123456" {
		t.Fatal("roundtrip failed")
	}
	if _, err = s.open(encrypted, "challenge-b"); err == nil {
		t.Fatal("ciphertext not bound to challenge")
	}
	if _, err = s.open(encrypted+"x", "challenge-a"); err == nil {
		t.Fatal("tampering accepted")
	}
	if s.digest("code", "a", "123456") == s.digest("code", "b", "123456") {
		t.Fatal("digest not scoped")
	}
}
func TestSecretsExcludedFromJSON(t *testing.T) {
	objects := []interface{}{Project{KeyHash: "sensitive"}, Challenge{Digest: "sensitive", PhoneCipher: "sensitive", Subject: "sensitive", IdempotencyKey: "sensitive"}, Job{Payload: "sensitive"}}
	for _, o := range objects {
		b, _ := json.Marshal(o)
		if bytes.Contains(b, []byte("sensitive")) {
			t.Fatalf("secret leaked: %T", o)
		}
	}
}
func TestConfigDisabledAndValidation(t *testing.T) {
	t.Setenv("OTP_ENABLED", "")
	if c, err := LoadConfig(); err != nil || c.Enabled {
		t.Fatal("disabled config failed")
	}
	t.Setenv("OTP_ENABLED", "true")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("missing secrets accepted")
	}
	t.Setenv("OTP_HMAC_KEY", strings.Repeat("11", 32))
	t.Setenv("OTP_ENCRYPTION_KEY", strings.Repeat("22", 32))
	t.Setenv("OTP_ADMIN_TOKEN", strings.Repeat("x", 40))
	t.Setenv("OTP_META_ACCESS_TOKEN", "")
	if _, err := LoadConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OTP_ENCRYPTION_KEY", strings.Repeat("11", 32))
	if _, err := LoadConfig(); err == nil {
		t.Fatal("same keys accepted")
	}
}

func TestIdentifiersUseCanonicalUUIDShape(t *testing.T) {
	if validID("urn:uuid:550e8400-e29b-41d4-a716-446655440000") || validID("550e8400e29b41d4a716446655440000") {
		t.Fatal("noncanonical identifier accepted")
	}
	if !validID("550e8400-e29b-41d4-a716-446655440000") {
		t.Fatal("valid UUID rejected")
	}
}
