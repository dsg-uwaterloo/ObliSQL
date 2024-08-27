package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"
)

// EncryptionEngine structure
type EncryptionEngine struct {
	encryptionString string
	encryptionKey    []byte
	iv               []byte
}

// NewEncryptionEngine initializes a new encryption engine with random keys
func NewEncryptionEngine() *EncryptionEngine {
	encryptionString := genRandom(32)
	iv := genRandom(16)
	return &EncryptionEngine{
		encryptionString: encryptionString,
		encryptionKey:    []byte(encryptionString),
		iv:               []byte(iv),
	}
}

// Encrypt encrypts the plaintext using AES-256-CBC
func (e *EncryptionEngine) Encrypt(plainText string) (string, error) {
	block, err := aes.NewCipher(e.encryptionKey)
	if err != nil {
		return "", err
	}

	plainBytes := []byte(plainText)
	blockSize := block.BlockSize()
	padding := blockSize - len(plainBytes)%blockSize
	plainBytes = append(plainBytes, bytes.Repeat([]byte{byte(padding)}, padding)...)

	cipherText := make([]byte, aes.BlockSize+len(plainBytes))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText[aes.BlockSize:], plainBytes)

	return hex.EncodeToString(cipherText), nil
}

// Decrypt decrypts the ciphertext using AES-256-CBC
func (e *EncryptionEngine) Decrypt(cipherText string) (string, error) {
	block, err := aes.NewCipher(e.encryptionKey)
	if err != nil {
		return "", err
	}

	cipherBytes, err := hex.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	if len(cipherBytes) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	iv := cipherBytes[:aes.BlockSize]
	cipherBytes = cipherBytes[aes.BlockSize:]

	if len(cipherBytes)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(cipherBytes, cipherBytes)

	// Unpad plaintext
	padding := int(cipherBytes[len(cipherBytes)-1])
	return string(cipherBytes[:len(cipherBytes)-padding]), nil
}

// HMAC computes a HMAC using SHA-256
func (e *EncryptionEngine) HMAC(key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(e.encryptionString))
	return hex.EncodeToString(h.Sum(nil))
}

// EncryptNonDeterministic encrypts the plaintext non-deterministically by appending random data
func (e *EncryptionEngine) EncryptNonDeterministic(plainText string) (string, error) {
	randomSuffix := genRandom(5)
	plainTextWithSuffix := plainText + "#" + randomSuffix
	return e.Encrypt(plainTextWithSuffix)
}

// DecryptNonDeterministic decrypts non-deterministic ciphertext and removes the random data
func (e *EncryptionEngine) DecryptNonDeterministic(cipherText string) (string, error) {
	decryptedText, err := e.Decrypt(cipherText)
	if err != nil {
		return "", err
	}
	return extractKey(decryptedText), nil
}

// ExtractKey extracts the key from the decrypted text
func extractKey(encryptedKey string) string {
	parts := strings.Split(encryptedKey, "#")
	if len(parts) > 1 {
		return parts[0]
	}
	return encryptedKey
}

// GetEncryptionString returns the encryption string
func (e *EncryptionEngine) GetEncryptionString() string {
	return e.encryptionString
}

func (e *EncryptionEngine) PRFEncrypt(key, plainText string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(plainText))
	hash := h.Sum(nil)

	// Return the raw binary data as a string (32 bytes)
	return string(hash)
}

// PRF computes the PRF using the stored encryption key
func (e *EncryptionEngine) PRF(plainText string) string {
	return e.PRFEncrypt(e.encryptionString, plainText)
}

// Generate a random string of a specific length
func genRandom(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		randomIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[randomIndex.Int64()]
	}
	return string(b)
}

func benchmarkEncryption(engine *EncryptionEngine, plainText string, targetDurationSeconds float64) {
	start := time.Now()
	end := start.Add(time.Duration(targetDurationSeconds * float64(time.Second)))

	iterations := 0
	totalBytesProcessed := 0
	plainTextSize := len(plainText)

	for time.Now().Before(end) {
		_, err := engine.Encrypt(plainText)
		if err != nil {
			fmt.Println("Encryption error:", err)
			return
		}
		iterations++
		totalBytesProcessed += plainTextSize
	}

	actualEnd := time.Now()
	elapsed := actualEnd.Sub(start)

	// Convert elapsed time to nanoseconds
	elapsedNs := float64(elapsed.Nanoseconds())
	// Calculate ns/op
	nsPerOp := elapsedNs / float64(iterations)
	// Calculate throughput (bytes per second)
	throughput := float64(totalBytesProcessed) / elapsed.Seconds()

	fmt.Println("Go Encryption:")
	fmt.Printf("  Iterations: %d\n", iterations)
	fmt.Printf("  Total time: %.6f seconds\n", elapsed.Seconds())
	fmt.Printf("  Time per operation: %.2f ns/op\n", nsPerOp)
	fmt.Printf("  Throughput: %.2f bytes/second\n", throughput)
}

func benchmarkDecryption(engine *EncryptionEngine, cipherText string, targetDurationSeconds float64) {
	start := time.Now()
	end := start.Add(time.Duration(targetDurationSeconds * float64(time.Second)))

	iterations := 0
	totalBytesProcessed := 0
	cipherTextSize := len(cipherText)

	for time.Now().Before(end) {
		_, err := engine.Decrypt(cipherText)
		if err != nil {
			fmt.Println("Decryption error:", err)
			return
		}
		iterations++
		totalBytesProcessed += cipherTextSize
	}

	actualEnd := time.Now()
	elapsed := actualEnd.Sub(start)

	// Convert elapsed time to nanoseconds
	elapsedNs := float64(elapsed.Nanoseconds())
	// Calculate ns/op
	nsPerOp := elapsedNs / float64(iterations)
	// Calculate throughput (bytes per second)
	throughput := float64(totalBytesProcessed) / elapsed.Seconds()

	fmt.Println("Go Decryption:")
	fmt.Printf("  Iterations: %d\n", iterations)
	fmt.Printf("  Total time: %.6f seconds\n", elapsed.Seconds())
	fmt.Printf("  Time per operation: %.2f ns/op\n", nsPerOp)
	fmt.Printf("  Throughput: %.2f bytes/second\n", throughput)
}

// func main() {
// 	engine := NewEncryptionEngine()
// 	plainText := "This is a test message for encryption."
// 	cipherText, err := engine.Encrypt(plainText)
// 	if err != nil {
// 		fmt.Println("Initial encryption failed:", err)
// 		return
// 	}

// 	// Target duration for the benchmark (in seconds)
// 	targetDurationSeconds := 10.0

// 	fmt.Printf("Benchmarking encryption for %.1f seconds:\n", targetDurationSeconds)
// 	benchmarkEncryption(engine, plainText, targetDurationSeconds)

// 	fmt.Printf("\nBenchmarking decryption for %.1f seconds:\n", targetDurationSeconds)
// 	benchmarkDecryption(engine, cipherText, targetDurationSeconds)
// }
