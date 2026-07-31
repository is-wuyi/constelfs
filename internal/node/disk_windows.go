//go:build windows

package node

// getTotalDiskSpace 获取总磁盘空间
func (a *Agent) getTotalDiskSpace() int64 {
	// TODO: Windows实现
	return 1024 * 1024 * 1024 * 100 // 默认100GB
}

// getDiskUsage 获取磁盘使用率
func (a *Agent) getDiskUsage() float64 {
	// TODO: Windows实现
	return 0
}
