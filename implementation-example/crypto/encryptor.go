package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "fmt"
    "io"
)

// TokenEncryptor 用于加密和解密 Token
type TokenEncryptor struct {
    key []byte
}

// NewTokenEncryptor 创建新的 Token 加密器
func NewTokenEncryptor(key string) (*TokenEncryptor, error) {
    if len(key) != 32 {
        return nil, errors.New("encryption key must be 32 bytes for AES-256")
    }
    return &TokenEncryptor{key: []byte(key)}, nil
}

// Encrypt 加密字符串
func (e *TokenEncryptor) Encrypt(plaintext string) (string, error) {
    if plaintext == "" {
        return "", errors.New("plaintext cannot be empty")
    }
    
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %w", err)
    }

    // 创建 GCM 模式
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("failed to create GCM: %w", err)
    }

    // 创建 nonce
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", fmt.Errorf("failed to generate nonce: %w", err)
    }

    // 加密数据
    plaintextBytes := []byte(plaintext)
    ciphertext := gcm.Seal(nonce, nonce, plaintextBytes, nil)

    // 返回 base64 编码的密文
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密字符串
func (e *TokenEncryptor) Decrypt(ciphertext string) (string, error) {
    if ciphertext == "" {
        return "", errors.New("ciphertext cannot be empty")
    }
    
    // base64 解码
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", fmt.Errorf("failed to decode base64: %w", err)
    }

    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %w", err)
    }

    // 创建 GCM 模式
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("failed to create GCM: %w", err)
    }

    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", errors.New("ciphertext too short")
    }

    // 提取 nonce 和密文
    nonce, ciphertext := data[:nonceSize], string(data[nonceSize:])

    // 解密数据
    plaintext, err := gcm.Open(nil, nonce, []byte(ciphertext), nil)
    if err != nil {
        return "", fmt.Errorf("failed to decrypt: %w", err)
    }

    return string(plaintext), nil
}

// GenerateKey 生成一个新的 32 字节加密密钥
func GenerateKey() (string, error) {
    key := make([]byte, 32)
    if _, err := io.ReadFull(rand.Reader, key); err != nil {
        return "", fmt.Errorf("failed to generate key: %w", err)
    }
    return base64.StdEncoding.EncodeToString(key), nil
}