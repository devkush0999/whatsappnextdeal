// Package otp provides project-scoped, single-use verification challenges.
package otp

import (
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"strconv"
)

type Config struct {
	Enabled                                                             bool
	AdminToken                                                          string
	HMACKey, EncryptionKey                                              []byte
	AllowSandbox, AllowEvolution                                        bool
	MetaToken, MetaPhoneID, MetaVersion, MetaAppSecret, MetaVerifyToken string
	RetentionDays                                                       int
}

func LoadConfig() (Config, error) {
	c := Config{Enabled: os.Getenv("OTP_ENABLED") == "true", AdminToken: os.Getenv("OTP_ADMIN_TOKEN"), AllowSandbox: os.Getenv("OTP_ALLOW_SANDBOX") == "true", AllowEvolution: os.Getenv("OTP_ALLOW_EVOLUTION") == "true", MetaToken: os.Getenv("OTP_META_ACCESS_TOKEN"), MetaPhoneID: os.Getenv("OTP_META_PHONE_ID"), MetaVersion: os.Getenv("OTP_META_VERSION"), MetaAppSecret: os.Getenv("OTP_META_APP_SECRET"), MetaVerifyToken: os.Getenv("OTP_META_VERIFY_TOKEN"), RetentionDays: 30}
	if !c.Enabled {
		return c, nil
	}
	var err error
	c.HMACKey, err = hex.DecodeString(os.Getenv("OTP_HMAC_KEY"))
	if err != nil || len(c.HMACKey) != 32 {
		return c, errors.New("OTP_HMAC_KEY must contain 64 hex characters")
	}
	c.EncryptionKey, err = hex.DecodeString(os.Getenv("OTP_ENCRYPTION_KEY"))
	if err != nil || len(c.EncryptionKey) != 32 {
		return c, errors.New("OTP_ENCRYPTION_KEY must contain 64 hex characters")
	}
	if len(c.AdminToken) < 32 {
		return c, errors.New("OTP_ADMIN_TOKEN must contain at least 32 characters")
	}
	if equal(c.HMACKey, c.EncryptionKey) {
		return c, errors.New("OTP encryption and HMAC keys must differ")
	}
	if s := os.Getenv("OTP_RETENTION_DAYS"); s != "" {
		c.RetentionDays, err = strconv.Atoi(s)
		if err != nil || c.RetentionDays < 1 || c.RetentionDays > 365 {
			return c, errors.New("OTP_RETENTION_DAYS must be 1–365")
		}
	}
	if c.MetaToken != "" && (!regexp.MustCompile(`^v\d+\.\d+$`).MatchString(c.MetaVersion) || !regexp.MustCompile(`^\d+$`).MatchString(c.MetaPhoneID) || len(c.MetaAppSecret) < 16 || len(c.MetaVerifyToken) < 16) {
		return c, errors.New("configure OTP_META_PHONE_ID, VERSION, APP_SECRET and VERIFY_TOKEN for Meta delivery")
	}
	return c, nil
}
