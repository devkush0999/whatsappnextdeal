package otp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
)

func equal(a, b []byte) bool { return hmac.Equal(a, b) }
func (s *Service) digest(domain string, values ...string) string {
	b, _ := json.Marshal(append([]string{domain}, values...))
	h := hmac.New(sha256.New, s.cfg.HMACKey)
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func randomSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func newCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}
func (s *Service) seal(value, context string) (string, error) {
	block, err := aes.NewCipher(s.cfg.EncryptionKey)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(value), []byte(context))), nil
}
func (s *Service) open(value, context string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(s.cfg.EncryptionKey)
	if err != nil {
		return "", err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(b) < g.NonceSize() {
		return "", fmt.Errorf("invalid ciphertext")
	}
	raw, err := g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], []byte(context))
	return string(raw), err
}
