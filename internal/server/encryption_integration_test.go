package server

import (
	"testing"
)

func TestEncryptionIntegrationGenerateKey(t *testing.T) {
	encMgr := NewEncryptionManager()
	ei := NewEncryptionIntegration(encMgr)
	
	// 生成密钥
	key, err := ei.GenerateKeyForFile("test-file-001")
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	
	// 验证密钥不为空
	if key == "" {
		t.Error("密钥不能为空")
	}
	
	// 验证密钥长度（Base64编码的32字节）
	if len(key) != 44 { // 32字节Base64编码后是44字符
		t.Errorf("密钥长度不匹配: 期望 %d, 实际 %d", 44, len(key))
	}
}

func TestEncryptionIntegrationGetKey(t *testing.T) {
	encMgr := NewEncryptionManager()
	ei := NewEncryptionIntegration(encMgr)
	
	// 生成密钥
	expectedKey, _ := ei.GenerateKeyForFile("test-file-002")
	
	// 获取密钥
	key, err := ei.GetKeyForFile("test-file-002")
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}
	
	// 验证密钥匹配
	if key != expectedKey {
		t.Errorf("密钥不匹配: 期望 %s, 实际 %s", expectedKey, key)
	}
	
	// 测试不存在的文件
	_, err = ei.GetKeyForFile("non-existent")
	if err == nil {
		t.Error("应该返回错误")
	}
}

func TestEncryptionIntegrationEncryptDecrypt(t *testing.T) {
	encMgr := NewEncryptionManager()
	ei := NewEncryptionIntegration(encMgr)
	
	// 生成密钥
	ei.GenerateKeyForFile("test-file-003")
	
	// 测试数据
	originalData := []byte("Hello, ConstelFS! This is a test file.")
	
	// 加密数据
	encryptedData, err := ei.EncryptData("test-file-003", originalData)
	if err != nil {
		t.Fatalf("加密数据失败: %v", err)
	}
	
	// 验证加密后数据不为空
	if len(encryptedData) == 0 {
		t.Error("加密后数据不能为空")
	}
	
	// 验证加密后数据与原始数据不同
	if string(encryptedData) == string(originalData) {
		t.Error("加密后数据应该与原始数据不同")
	}
	
	// 解密数据
	decryptedData, err := ei.DecryptData("test-file-003", encryptedData)
	if err != nil {
		t.Fatalf("解密数据失败: %v", err)
	}
	
	// 验证解密后数据与原始数据相同
	if string(decryptedData) != string(originalData) {
		t.Errorf("解密后数据不匹配: 期望 %s, 实际 %s", string(originalData), string(decryptedData))
	}
}

func TestEncryptionIntegrationDeleteKey(t *testing.T) {
	encMgr := NewEncryptionManager()
	ei := NewEncryptionIntegration(encMgr)
	
	// 生成密钥
	ei.GenerateKeyForFile("test-file-004")
	
	// 验证密钥存在
	_, err := ei.GetKeyForFile("test-file-004")
	if err != nil {
		t.Fatalf("密钥应该存在: %v", err)
	}
	
	// 删除密钥
	ei.DeleteKeyForFile("test-file-004")
	
	// 验证密钥已删除
	_, err = ei.GetKeyForFile("test-file-004")
	if err == nil {
		t.Error("密钥应该已被删除")
	}
}

func TestEncryptionIntegrationExportImportKey(t *testing.T) {
	encMgr := NewEncryptionManager()
	ei := NewEncryptionIntegration(encMgr)
	
	// 生成密钥
	originalKey, _ := ei.GenerateKeyForFile("test-file-005")
	
	// 导出密钥
	exportedKey, err := ei.ExportKey("test-file-005")
	if err != nil {
		t.Fatalf("导出密钥失败: %v", err)
	}
	
	// 验证导出的密钥不为空
	if exportedKey == "" {
		t.Error("导出的密钥不能为空")
	}
	
	// 删除原密钥
	ei.DeleteKeyForFile("test-file-005")
	
	// 导入密钥
	err = ei.ImportKey("test-file-005", exportedKey)
	if err != nil {
		t.Fatalf("导入密钥失败: %v", err)
	}
	
	// 验证导入的密钥与原密钥相同
	importedKey, _ := ei.GetKeyForFile("test-file-005")
	if importedKey != originalKey {
		t.Errorf("导入的密钥不匹配: 期望 %s, 实际 %s", originalKey, importedKey)
	}
}
