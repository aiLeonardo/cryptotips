package lib

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sevlyar/go-daemon"
)

var daemonCtx *daemon.Context

func newDaemonCtx(cmdName string) *daemon.Context {
	if daemonCtx != nil {
		return daemonCtx
	}

	filename, err := os.Executable()
	if err != nil {
		fmt.Printf("读取工作目录出错: %v \n", err)
		return nil
	}

	dirname := filepath.Dir(filename)
	pidfile := filepath.Join(dirname, "logs", fmt.Sprintf("app.%s.pid", cmdName))
	logfile := filepath.Join(dirname, "logs", fmt.Sprintf("app.%s.error", cmdName))

	daemonCtx = &daemon.Context{
		PidFileName: pidfile,
		PidFilePerm: 0644,
		LogFileName: logfile,
		LogFilePerm: 0640,
		WorkDir:     dirname,
		Umask:       022,
		Args:        os.Args,
	}

	return daemonCtx
}

func DaemonStart(cmdName string) *daemon.Context {
	daemonCtx := newDaemonCtx(cmdName)
	daemonProcess, err := daemonCtx.Reborn()
	if err != nil {
		fmt.Printf("Failed to start: %v", err)
		return nil
	}
	if daemonProcess != nil {
		fmt.Println("Daemon started successfully.")
		fmt.Printf("sub process: %d \n", daemonProcess.Pid)
		os.Exit(0)
		return nil
	}

	return daemonCtx
}

func WaitSIGTERM(mainEndSign <-chan struct{}) {
	logger := LoadLogger()
	// --- 捕获信号并优雅退出 ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case <-quit:
		// 收到信号后，执行清理逻辑
		logger.Infoln("接收到退出信号，正在关闭服务...")
	case <-mainEndSign:
		logger.Infoln("任务顺利完成，正在关闭服务...")
	}
}
