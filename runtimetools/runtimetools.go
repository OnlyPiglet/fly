package runtimetools

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
)

func Go(fn func()) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println(err)
				debug.PrintStack()
			}
		}()

		fn()
	}()
}

// GetCallStack 获取调用栈信息（跳过指定层数）
func GetCallStack(skip int) string {
	pc := make([]uintptr, 10) // 最多10层
	n := runtime.Callers(skip, pc)
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pc[:n])
	var stack strings.Builder

	for {
		frame, more := frames.Next()
		// 跳过slog包内部的调用
		if !strings.Contains(frame.Function, "log/slog") {
			file := frame.File[strings.LastIndex(frame.File, "/")+1:]
			stack.WriteString(file)
			stack.WriteString(":")
			stack.WriteString(strconv.Itoa(frame.Line))
			if more {
				stack.WriteString(" <- ")
			}
		}
		if !more {
			break
		}
	}

	return stack.String()
}
