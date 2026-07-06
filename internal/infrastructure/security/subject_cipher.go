package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const subjectCipherVersion = "v1."

type SubjectCipher struct {
	aead cipher.AEAD
}

func NewSubjectCipher(encodedKey string) (*SubjectCipher, error) {
	key, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, errors.New("JWT_SUB_ENCRYPTION_KEY must be valid base64")
	}
	if len(key) != 32 {
		return nil, errors.New("JWT_SUB_ENCRYPTION_KEY must decode to 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create AES-GCM cipher: %w", err)
	}

	return &SubjectCipher{aead: aead}, nil
}

func (c *SubjectCipher) Encrypt(subject string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate subject nonce: %w", err)
	}

	sealed := c.aead.Seal(nonce, nonce, []byte(subject), nil)
	return subjectCipherVersion + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *SubjectCipher) Decrypt(encryptedSubject string) (string, error) {
	if !strings.HasPrefix(encryptedSubject, subjectCipherVersion) {
		return "", errors.New("unsupported encrypted subject version")
	}

	sealed, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encryptedSubject, subjectCipherVersion))
	if err != nil {
		return "", errors.New("invalid encrypted subject")
	}
	if len(sealed) < c.aead.NonceSize() {
		return "", errors.New("invalid encrypted subject")
	}

	nonce, ciphertext := sealed[:c.aead.NonceSize()], sealed[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("invalid encrypted subject")
	}

	return string(plaintext), nil
}
