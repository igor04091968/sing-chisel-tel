package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

// Cipher provides AES-CBC encryption and decryption with PKCS#7 padding.
type Cipher struct {
	block cipher.Block
}

// NewCipher creates a new Cipher using a pre-shared key.
// The key is hashed with SHA-256 to ensure it's the correct size for AES-256.
func NewCipher(key []byte) (*Cipher, error) {
	// Use SHA-256 to derive a 32-byte key for AES-256.
	hash := sha256.Sum256(key)
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, err
	}
	return &Cipher{block: block}, nil
}

// Encrypt encrypts plaintext using AES-CBC with PKCS#7 padding.
// The IV is randomly generated and prepended to the ciphertext.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	// Pad the plaintext to a multiple of the block size.
	paddedPlaintext, err := pkcs7Pad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, err
	}

	// The IV needs to be unique, but not secure.
	// So it's common to include it at the beginning of the ciphertext.
	ciphertext := make([]byte, aes.BlockSize+len(paddedPlaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCBCEncrypter(c.block, iv)
	stream.CryptBlocks(ciphertext[aes.BlockSize:], paddedPlaintext)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-CBC and removes PKCS#7 padding.
// It assumes the IV is prepended to the ciphertext.
func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	stream := cipher.NewCBCDecrypter(c.block, iv)

	// CryptBlocks can work in-place if the two arguments are the same.
	stream.CryptBlocks(ciphertext, ciphertext)

	// Unpad the decrypted plaintext.
	return pkcs7Unpad(ciphertext, aes.BlockSize)
}

// pkcs7Pad pads the given plaintext to a multiple of blockSize.
func pkcs7Pad(plaintext []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 {
		return nil, errors.New("invalid block size")
	}
	padding := blockSize - (len(plaintext) % blockSize)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(plaintext, padtext...), nil
}

// pkcs7Unpad removes PKCS#7 padding from the given decrypted data.
func pkcs7Unpad(paddedData []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 {
		return nil, errors.New("invalid block size")
	}
	if len(paddedData) == 0 {
		return nil, errors.New("padded data is empty")
	}
	if len(paddedData)%blockSize != 0 {
		return nil, errors.New("padded data is not a multiple of the block size")
	}

	paddingLen := int(paddedData[len(paddedData)-1])
	if paddingLen > len(paddedData) || paddingLen > blockSize {
		return nil, errors.New("invalid padding")
	}

	return paddedData[:len(paddedData)-paddingLen], nil
}
