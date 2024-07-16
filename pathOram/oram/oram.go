package oram

import (
	mathrand "math/rand"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	// "encoding/json"
	"errors"
	// "fmt"
	// "strings"
	"io"
    // "github.com/go-redis/redis/v8" // using go-redis library
	// "context"
)

// Block represents a key-value pair
type Block struct {
	BlockId int
	Key   int    // dummy can have key -1
	Value string
}

// Bucket represents a collection of blocks
type Bucket struct {
	Blocks        []Block
	RealBlockCount int
}

// ############################### Cryptography functions #######################################

func getRandomInt(max int) int {
    return mathrand.Intn(max)
}

func generateRandomKey() ([]byte, error) {
	key := make([]byte, 32) // AES-256
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

func decrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(data) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}
