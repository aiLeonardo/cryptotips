package lib

import (
	"net"
)

// GetLocalIP 通过 UDP 获取默认网络接口的 IP（非 127.0.0.1）
func GetLocalIP() string {
	logger := LoadLogger()
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		logger.Errorf("GetLocalIP error: %v", err)
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
