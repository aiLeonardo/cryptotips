package tgclient

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var floodWaitRe = regexp.MustCompile(`FLOOD_WAIT_(\d+)`)

func floodWait(err error) time.Duration {
	if err == nil {
		return 0
	}
	s := strings.ToUpper(err.Error())
	m := floodWaitRe.FindStringSubmatch(s)
	if len(m) != 2 {
		return 0
	}
	sec, _ := strconv.Atoi(m[1])
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}
