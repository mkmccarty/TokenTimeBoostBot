package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// The size of the AES key. 32 bytes for AES-256.
const keySize = 32

// GenerateKey creates a new, random 32-byte key for AES-256.
func GenerateKey() ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// EncryptAndCombine performs AES-GCM encryption and returns the
// nonce and ciphertext combined into a single byte slice.
func EncryptAndCombine(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM instance: %w", err)
	}

	// Create a new, unique nonce.
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal the plaintext, which returns the ciphertext.
	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)

	// Prepend the nonce to the ciphertext.
	combined := append(nonce, ciphertext...)

	return combined, nil
}

// DecryptCombined performs AES-GCM decryption on a combined byte slice.
// It splits the nonce from the ciphertext and then decrypts the data.
func DecryptCombined(key []byte, combined []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM instance: %w", err)
	}

	nonceSize := aesgcm.NonceSize()
	if len(combined) < nonceSize {
		return nil, fmt.Errorf("invalid combined data: too short to contain nonce")
	}

	// Split the combined data into nonce and ciphertext.
	nonce := combined[:nonceSize]
	ciphertext := combined[nonceSize:]

	// Open the data.
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt or authenticate data: %w", err)
	}

	return plaintext, nil
}
