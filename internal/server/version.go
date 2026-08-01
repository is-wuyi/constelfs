package server

import (
	"fmt"
	"log"
	"sort"
	"time"
)

// FileInfo 文件信息
type FileInfo struct {
	FileID          string    `json:"file_id"`
	FileName        string    `json:"file_name"`
	FilePath        string    `json:"file_path"`
	IsDirectory     bool      `json:"is_directory"`
	LatestVersion   int       `json:"latest_version"`
	VersionCount    int       `json:"version_count"`
	MaxVersions     int       `json:"max_versions"`
	EncryptionKey   string    `json:"encryption_key"`
	Size            int64     `json:"size"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// FileVersion 文件版本
type FileVersion struct {
	VersionID   string    `json:"version_id"`
	FileID      string    `json:"file_id"`
	Version     int       `json:"version"`
	Size        int64     `json:"size"`
	Hash        string    `json:"hash"`
	ChunkIDs    []string  `json:"chunk_ids"`
	NodeIDs     []string  `json:"node_ids"`
	CreatedAt   time.Time `json:"created_at"`
}

// VersionManager 版本管理器
type VersionManager struct {
	storage *StorageManager
}

// NewVersionManager 创建版本管理器
func NewVersionManager(storage *StorageManager) *VersionManager {
	return &VersionManager{
		storage: storage,
	}
}

// CreateNewVersion 创建新版本
func (vm *VersionManager) CreateNewVersion(file *FileInfo, chunkIDs []string, nodeIDs []string, size int64, hash string) (*FileVersion, error) {
	// 创建新版本
	newVersion := &FileVersion{
		VersionID: generateVersionID(),
		FileID:    file.FileID,
		Version:   file.LatestVersion + 1,
		Size:      size,
		Hash:      hash,
		ChunkIDs:  chunkIDs,
		NodeIDs:   nodeIDs,
		CreatedAt: time.Now(),
	}

	// 更新文件信息
	file.LatestVersion = newVersion.Version
	file.VersionCount++
	file.Size = size
	file.UpdatedAt = time.Now()

	log.Printf("创建新版本: 文件=%s, 版本=%d", file.FileID, newVersion.Version)

	return newVersion, nil
}

// RollbackToVersion 回滚到历史版本
func (vm *VersionManager) RollbackToVersion(file *FileInfo, targetVersion *FileVersion) (*FileVersion, error) {
	// 创建新版本，内容与目标版本相同
	newVersion := &FileVersion{
		VersionID: generateVersionID(),
		FileID:    file.FileID,
		Version:   file.LatestVersion + 1,
		Size:      targetVersion.Size,
		Hash:      targetVersion.Hash,
		ChunkIDs:  targetVersion.ChunkIDs, // 复用分片
		NodeIDs:   targetVersion.NodeIDs,
		CreatedAt: time.Now(),
	}

	// 更新文件信息
	file.LatestVersion = newVersion.Version
	file.VersionCount++
	file.Size = targetVersion.Size
	file.UpdatedAt = time.Now()

	log.Printf("回滚版本: 文件=%s, 目标版本=%d, 新版本=%d", file.FileID, targetVersion.Version, newVersion.Version)

	return newVersion, nil
}

// CleanupOldVersions 清理旧版本
func (vm *VersionManager) CleanupOldVersions(file *FileInfo, versions []*FileVersion) ([]*FileVersion, error) {
	if len(versions) <= file.MaxVersions {
		return versions, nil
	}

	// 按版本号排序
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version < versions[j].Version
	})

	// 需要删除的版本
	versionsToDelete := versions[:len(versions)-file.MaxVersions]
	versionsToKeep := versions[len(versions)-file.MaxVersions:]

	// 删除旧版本
	for _, version := range versionsToDelete {
		if err := vm.DeleteVersion(version); err != nil {
			log.Printf("删除版本失败: %v", err)
			continue
		}
		file.VersionCount--
	}

	log.Printf("清理旧版本: 文件=%s, 删除=%d, 保留=%d", file.FileID, len(versionsToDelete), len(versionsToKeep))

	return versionsToKeep, nil
}

// DeleteVersion 删除版本
func (vm *VersionManager) DeleteVersion(version *FileVersion) error {
	// 删除分片数据
	for _, chunkID := range version.ChunkIDs {
		if err := vm.storage.DeleteChunk(chunkID); err != nil {
			log.Printf("删除分片失败: %v", err)
		}
	}

	log.Printf("删除版本: 版本=%s, 分片数=%d", version.VersionID, len(version.ChunkIDs))

	return nil
}

// generateVersionID 生成版本ID
func generateVersionID() string {
	return fmt.Sprintf("v_%d", time.Now().UnixNano())
}
