// otp-secrets writes fresh environment values to stdout; redirect into a protected file.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func secret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func main() {
	fmt.Println("OTP_ENABLED=true")
	fmt.Println("OTP_ALLOW_SANDBOX=false")
	fmt.Println("OTP_ALLOW_EVOLUTION=false")
	for _, name := range []string{"OTP_ADMIN_TOKEN", "OTP_HMAC_KEY", "OTP_ENCRYPTION_KEY"} {
		fmt.Printf("%s=%s\n", name, secret())
	}
}
