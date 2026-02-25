package lib

import (
	_ "image/jpeg"
	_ "image/png"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type timeFormat struct {
	logger *logrus.Logger
}

func NewTimeFormat() *timeFormat {
	return &timeFormat{
		logger: LoadLogger(),
	}
}

// randomDuration 生成 [minMs, maxMs] 范围内的随机时间
func (mh *timeFormat) RandomDuration(minMs, maxMs int) time.Duration {
	diff := maxMs - minMs
	return time.Duration(minMs + rand.Intn(diff))
}

// TimeStrToSeconds 将时间字符串转为秒数
func (mh *timeFormat) TimeStrToSeconds(timeStr string) int {
	parts := strings.Split(strings.TrimSpace(timeStr), ":")
	intParts := []int{}
	for _, p := range parts {
		num, err := strconv.Atoi(p)
		if err != nil {
			mh.logger.Errorf("TimeStrToSeconds strconv Atoi error: %v", err)
			return 0
		}
		intParts = append(intParts, num)
	}
	if len(intParts) == 2 {
		return intParts[0]*60 + intParts[1]
	} else if len(intParts) == 3 {
		return intParts[0]*3600 + intParts[1]*60 + intParts[2]
	}
	return 0
}

func (mh *timeFormat) IsAllDigit(origin string) bool {
	_, err := strconv.ParseUint(origin, 10, 64)
	return err == nil
}
