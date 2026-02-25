package lib

import (
	"sync"

	"github.com/sasha-s/go-deadlock"
)

var rwmutexForHttp *deadlock.Mutex
var synconceForHttp sync.Once // 控制只初始化一次

func MakeHttpLock() *deadlock.Mutex {
	synconceForHttp.Do(func() {
		rwmutexForHttp = &deadlock.Mutex{}
	})

	return rwmutexForHttp
}
