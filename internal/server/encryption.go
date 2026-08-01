package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// EncryptionManager 加密管理器
type EncryptionManager struct {
	// 密钥存储
	keys map[string]string
}

// NewEncryptionManager 创建加密管理器
func NewEncryptionManager() *EncryptionManager {
	return &EncryptionManager{
		keys: make(map[string]string),
	}
}

// GenerateKey 生成AES-256密钥
func (em *EncryptionManager) GenerateKey() (string, error) {
	key := make([]byte, 32) // AES-256需要32字节
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("生成密钥失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// Encrypt 加密数据
func (em *EncryptionManager) Encrypt(data []byte, keyStr string) ([]byte, error) {
	// 解码密钥
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("解码密钥失败: %w", err)
	}

	// 创建AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密块失败: %w", err)
	}

	// 使用GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %w", err)
	}

	// 生成随机数
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成随机数失败: %w", err)
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

// Decrypt 解密数据
func (em *EncryptionManager) Decrypt(ciphertext []byte, keyStr string) ([]byte, error) {
	// 解码密钥
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("解码密钥失败: %w", err)
	}

	// 创建AES加密块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建加密块失败: %w", err)
	}

	// 使用GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %w", err)
	}

	// 检查数据长度
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文太短")
	}

	// 分离随机数和密文
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}

// SaveKey 保存密钥
func (em *EncryptionManager) SaveKey(fileID, key string) {
	em.keys[fileID] = key
}

// GetKey 获取密钥
func (em *EncryptionManager) GetKey(fileID string) (string, bool) {
	key, exists := em.keys[fileID]
	return key, exists
}

// DeleteKey 删除密钥
func (em *EncryptionManager) DeleteKey(fileID string) {
	delete(em.keys, fileID)
}
