package lib

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// 磁盘空间信息
type DiskSpaceInfo struct {
	Total    uint64 // 总空间
	SysFree  uint64 // 可用空间
	Used     uint64 // 已使用空间
	UserFree uint64 // 可供用户使用的空间
}

type SystemStats struct {
	ticker *time.Ticker
	logger *logrus.Logger
}

func NewSystemStats() *SystemStats {
	var tickerTime int = viper.GetInt("systemstats.ticker")
	if tickerTime < 5 {
		tickerTime = 10
	}

	s := &SystemStats{
		ticker: time.NewTicker(time.Duration(tickerTime) * time.Second),
		logger: LoadLogger(),
	}

	go s.tickerSystemStatus()
	return s
}

func (s *SystemStats) StopTicker() {
	s.ticker.Stop()
}

func (s *SystemStats) tickerSystemStatus() {
	for range s.ticker.C {
		s.printSystemStats()
	}
}

func (s *SystemStats) printSystemStats() {
	var warnPercent int = viper.GetInt("systemstats.used_percent_warn")
	var enableWarn bool = false
	var output string
	// CPU 使用率
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		s.logger.Errorf("获取CPU信息失败: %v", err)
		return
	}
	output = fmt.Sprintf("系统CPU使用率: %.2f%%", cpuPercent[0])

	// 内存信息
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		s.logger.Errorf("获取内存信息失败: %v", err)
		return
	}
	if warnPercent > 0 && memInfo.UsedPercent > float64(warnPercent) {
		enableWarn = true
	}
	output = fmt.Sprintf("%s, 系统内存使用率: %.2f%%", output, memInfo.UsedPercent)
	output = fmt.Sprintf("%s, 系统可用内存: %d MB", output, memInfo.Available/1024/1024)

	// 当前进程信息
	pid := os.Getpid()
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		s.logger.Errorf("获取进程信息失败: %v", err)
		return
	}

	output = fmt.Sprintf("%s, 进程PID: %d", output, pid)

	cpuPercentProcess, _ := p.CPUPercent()
	memInfoProcess, _ := p.MemoryInfo()

	output = fmt.Sprintf("%s, 进程CPU使用率: %.2f%%", output, cpuPercentProcess)
	output = fmt.Sprintf("%s, 进程内存使用: %d MB", output, memInfoProcess.RSS/1024/1024)
	output = fmt.Sprintf("%s, Goroutine数量: %d", output, runtime.NumGoroutine())

	dirPath := viper.GetString("systemstats.dir_for_files")
	dsInfo, err := s.GetDiskSpace(dirPath)
	if err != nil {
		s.logger.Errorf("获取磁盘信息失败: %v", err)
		return
	}

	// 打印磁盘可用空间信息
	output = fmt.Sprintf("%s, Disk 已用:%s/%s, 剩余空间: %s", output, s.FormatBytes(dsInfo.Used), s.FormatBytes(dsInfo.Total), s.FormatBytes(dsInfo.UserFree))
	if warnPercent > 0 {
		diskUsedPercent := math.Round(float64(dsInfo.Used) / float64(dsInfo.Total) * 100.0)
		if diskUsedPercent > float64(warnPercent) {
			enableWarn = true
		}
	}

	if enableWarn {
		s.logger.Warnln(output)
		return
	}

	s.logger.Infoln(output)
}

// 格式化字节大小
func (s *SystemStats) FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
