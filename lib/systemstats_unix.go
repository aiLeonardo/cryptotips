//go:build linux || darwin
// +build linux darwin

package lib

import (
	"errors"
	"strings"
	"syscall"
)

// 获取磁盘空间信息（根据操作系统动态选择）
// 获取磁盘空间信息 - Linux/macOS
func (s *SystemStats) GetDiskSpace(dirPath string) (*DiskSpaceInfo, error) {
	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "" {
		return nil, errors.New("未指定检测目录")
	}

	var stat syscall.Statfs_t
	err := syscall.Statfs(dirPath, &stat)
	if err != nil {
		return nil, err
	}

	// 计算空间信息
	blockSize := uint64(stat.Bsize)
	total := stat.Blocks * blockSize
	free := stat.Bavail * blockSize
	used := total - (stat.Bfree * blockSize)

	return &DiskSpaceInfo{
		Total:    total,
		SysFree:  stat.Bfree * blockSize,
		UserFree: free,
		Used:     used,
	}, nil
}
