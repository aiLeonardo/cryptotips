//go:build windows
// +build windows

package lib

import (
	"errors"
	"strings"

	"github.com/shirou/gopsutil/disk"
)

// 获取磁盘空间信息（根据操作系统动态选择）
// 获取磁盘空间信息 - Windows 版本
func (s *SystemStats) GetDiskSpace(dirPath string) (*DiskSpaceInfo, error) {
	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "" {
		return nil, errors.New("未指定检测目录")
	}

	// Windows 使用 gopsutil 获取磁盘信息
	diskUsage, err := disk.Usage(dirPath)
	if err != nil {
		return nil, err
	}

	return &DiskSpaceInfo{
		Total:    diskUsage.Total,
		Used:     diskUsage.Used,
		SysFree:  diskUsage.Free,
		UserFree: diskUsage.Free,
	}, nil
}
