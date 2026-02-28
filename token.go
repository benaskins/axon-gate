package gate

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

func GenerateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("ap_%d_%s", time.Now().Unix(), base64.RawURLEncoding.EncodeToString(b))
}

func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
