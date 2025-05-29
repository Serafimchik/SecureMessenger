package utils

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/lestrrat-go/jwx/v2/jwk"
)

func ParsePublicKeyFromBase64JWK(encoded string) (*rsa.PublicKey, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Base64 JWK: %w", err)
	}

	key, err := jwk.ParseKey(decoded)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWK key: %w", err)
	}

	var pubKey crypto.PublicKey
	if err := key.Raw(&pubKey); err != nil {
		return nil, fmt.Errorf("failed to extract public key: %w", err)
	}

	rsaKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA")
	}

	return rsaKey, nil
}

func EncryptAESKeyWithRSA(rawAESKeyBase64 string, rsaPubKey *rsa.PublicKey) (string, error) {
	aesKeyBytes, err := base64.StdEncoding.DecodeString(rawAESKeyBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode AES key base64: %w", err)
	}

	encryptedBytes, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPubKey, aesKeyBytes, nil)
	if err != nil {
		return "", fmt.Errorf("RSA encryption failed: %w", err)
	}

	encryptedKeyBase64 := base64.StdEncoding.EncodeToString(encryptedBytes)
	return encryptedKeyBase64, nil
}
