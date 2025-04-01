package aws_secrets

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func GetAwsSecrets(secretId string, optFns ...func(*config.LoadOptions) error) (string, error) {
	// 1. Initialize AWS configuration (auto loading IAM role credentials)
	cfg, err := config.LoadDefaultConfig(context.TODO(), optFns...)
	if err != nil {
		return "", err
	}

	// 2. Get the Secrets Manager client
	secretsClient := secretsmanager.NewFromConfig(cfg)

	// 3. Get encrypted Secret from Secrets Manager
	getSecretInput := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretId),
	}
	secretOutput, err := secretsClient.GetSecretValue(context.TODO(), getSecretInput)
	if err != nil {
		return "", errors.New(fmt.Sprintf("failed to obtain secret: %s", err))
	}

	// 4. Decode the encrypted Secret
	return *secretOutput.SecretString, nil
}

// AESEncrypt Encryption function (returns base64-encoded "IV + ciphertext")
func AESEncrypt(plaintext, key string) (string, error) {
	// 1. Verify key length (32 bytes required for AES-256)
	if len(key) != 32 {
		return "", errors.New("the key must be 32 bytes (AES-256)")
	}

	// 2. Generate random IV (16 bytes)
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %v", err)
	}

	// 3. Create an AES Encryptor
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to create an AES encryptor: %v", err)
	}

	// 4. Add PKCS7 fill
	var plaintextPad = pkcs7Pad([]byte(plaintext), aes.BlockSize)

	// 5. Execute encryption (IV + ciphertext)
	ciphertext := make([]byte, len(plaintextPad))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintextPad)

	// 6. Combine IV and ciphertext and convert to Base64
	combined := append(iv, ciphertext...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

// AESDecrypt Decryption function (input is "IV + ciphertext" encoded by base64)
func AESDecrypt(encodedText, key string) (string, error) {
	// 1. Decode Base64
	combined, err := base64.StdEncoding.DecodeString(encodedText)
	if err != nil {
		return "", fmt.Errorf("base64 decoding failed: %v", err)
	}

	// 2. Separation of IV and ciphertext
	if len(combined) < aes.BlockSize {
		return "", errors.New("invalid ciphertext length")
	}
	iv := combined[:aes.BlockSize]
	ciphertext := combined[aes.BlockSize:]

	// 3. Create an AES decryptor
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to create the AES decryptor: %v", err)
	}

	// 4. Execute decryption
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 5. Remove PKCS7 fill
	return string(pkcs7Unpad(plaintext)), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	padding := int(data[len(data)-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil
	}
	return data[:len(data)-padding]
}
