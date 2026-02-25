package lib

import (
	"fmt"
	rdebug "runtime/debug"

	"github.com/sirupsen/logrus"
)

func RecoverInfo() {
	if r := recover(); r != nil {
		fmt.Printf("程序运行异常: %v \n", r)
		// 打印堆栈信息
		rdebug.PrintStack()
	}
}

func RecoverLogMsg(logger *logrus.Logger) {
	if r := recover(); r != nil {
		// 打印堆栈信息
		stack := string(rdebug.Stack())
		logger.Errorf("程序运行异常: %v, stack: %s", r, stack)

	}
}
