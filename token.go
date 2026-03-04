package gate

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

func GenerateID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("ap_%d_%s", time.Now().Unix(), base64.RawURLEncoding.EncodeToString(b)), nil
}

func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
