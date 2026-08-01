package server

import (
	"encoding/base64"
	"fmt"
	"log"
	"sync"
)

// EncryptionIntegration 加密集成
type EncryptionIntegration struct {
	encMgr *EncryptionManager
	keys   map[string]string // fileID -> key
	mu     sync.RWMutex
}

// NewEncryptionIntegration 创建加密集成
func NewEncryptionIntegration(encMgr *EncryptionManager) *EncryptionIntegration {
	return &EncryptionIntegration{
		encMgr: encMgr,
		keys:   make(map[string]string),
	}
}

// GenerateKeyForFile 为文件生成加密密钥
func (ei *EncryptionIntegration) GenerateKeyForFile(fileID string) (string, error) {
	ei.mu.Lock()
	defer ei.mu.Unlock()
	
	// 生成密钥
	key, err := ei.encMgr.GenerateKey()
	if err != nil {
		return "", fmt.Errorf("生成密钥失败: %w", err)
	}
	
	// 保存密钥
	ei.keys[fileID] = key
	
	log.Printf("为文件 %s 生成加密密钥", fileID)
	
	return key, nil
}

// GetKeyForFile 获取文件的加密密钥
func (ei *EncryptionIntegration) GetKeyForFile(fileID string) (string, error) {
	ei.mu.RLock()
	defer ei.mu.RUnlock()
	
	key, exists := ei.keys[fileID]
	if !exists {
		return "", fmt.Errorf("文件 %s 的加密密钥不存在", fileID)
	}
	
	return key, nil
}

// EncryptData 加密数据
func (ei *EncryptionIntegration) EncryptData(fileID string, data []byte) ([]byte, error) {
	// 获取密钥
	key, err := ei.GetKeyForFile(fileID)
	if err != nil {
		return nil, err
	}
	
	// 加密数据
	encryptedData, err := ei.encMgr.Encrypt(data, key)
	if err != nil {
		return nil, fmt.Errorf("加密数据失败: %w", err)
	}
	
	log.Printf("加密数据成功: 文件 %s, 原始大小 %d, 加密后大小 %d", fileID, len(data), len(encryptedData))
	
	return encryptedData, nil
}

// DecryptData 解密数据
func (ei *EncryptionIntegration) DecryptData(fileID string, encryptedData []byte) ([]byte, error) {
	// 获取密钥
	key, err := ei.GetKeyForFile(fileID)
	if err != nil {
		return nil, err
	}
	
	// 解密数据
	data, err := ei.encMgr.Decrypt(encryptedData, key)
	if err != nil {
		return nil, fmt.Errorf("解密数据失败: %w", err)
	}
	
	log.Printf("解密数据成功: 文件 %s, 加密大小 %d, 解密后大小 %d", fileID, len(encryptedData), len(data))
	
	return data, nil
}

// DeleteKeyForFile 删除文件的加密密钥
func (ei *EncryptionIntegration) DeleteKeyForFile(fileID string) {
	ei.mu.Lock()
	defer ei.mu.Unlock()
	
	delete(ei.keys, fileID)
	
	log.Printf("删除文件 %s 的加密密钥", fileID)
}

// ExportKey 导出密钥（用于Web界面显示）
func (ei *EncryptionIntegration) ExportKey(fileID string) (string, error) {
	key, err := ei.GetKeyForFile(fileID)
	if err != nil {
		return "", err
	}
	
	// 返回Base64编码的密钥
	return base64.StdEncoding.EncodeToString([]byte(key)), nil
}

// ImportKey 导入密钥
func (ei *EncryptionIntegration) ImportKey(fileID string, keyBase64 string) error {
	// 解码Base64
	keyBytes, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return fmt.Errorf("解码密钥失败: %w", err)
	}
	
	ei.mu.Lock()
	defer ei.mu.Unlock()
	
	ei.keys[fileID] = string(keyBytes)
	
	log.Printf("导入文件 %s 的加密密钥", fileID)
	
	return nil
}
