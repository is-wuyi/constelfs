//go:build linux || darwin

package node

import "syscall"

// getTotalDiskSpace 获取总磁盘空间
func (a *Agent) getTotalDiskSpace() int64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(a.config.StoragePath, &stat); err != nil {
		return 1024 * 1024 * 1024 * 100 // 默认100GB
	}
	return int64(stat.Blocks * uint64(stat.Bsize))
}

// getDiskUsage 获取磁盘使用率
func (a *Agent) getDiskUsage() float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(a.config.StoragePath, &stat); err != nil {
		return 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	return float64(used) / float64(total) * 100
}
