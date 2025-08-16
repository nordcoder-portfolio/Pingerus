package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var ErrTokenInvalid = errors.New("invalid token")

func ParseAndValidate(token string, secret []byte) (*AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}
	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]

	signingInput := headerB64 + "." + payloadB64
	expectedSig := hmacSHA256(secret, []byte(signingInput))
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	if !hmac.Equal(sig, expectedSig) {
		return nil, ErrTokenInvalid
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims AccessClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("unmarshal claims: %w", err)
	}

	now := time.Now().Unix()
	if claims.Iat > now {
		return nil, errors.New("token used before issued")
	}
	if claims.Exp < now {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

func (c AccessClaims) SignedString(secret []byte) (string, error) {
	header := base64URL([]byte(`{"alg":"HS256","typ":"JWT"}`))

	payloadJSON, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	payload := base64URL(payloadJSON)

	sigInput := header + "." + payload
	sig := hmacSHA256(secret, []byte(sigInput))

	return sigInput + "." + base64URL(sig), nil
}

func GenerateRawToken(nBytes int) (raw string, err error) {
	b := make([]byte, nBytes)
	if _, err = rand.Read(b); err != nil {
		return "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, nil
}

func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func base64URL(data []byte) string {
	s := base64.URLEncoding.EncodeToString(data)
	return trimRight(s, '=')
}

func hmacSHA256(secret, message []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(message)
	return mac.Sum(nil)
}

func trimRight(s string, c byte) string {
	i := len(s)
	for i > 0 && s[i-1] == c {
		i--
	}
	return s[:i]
}
