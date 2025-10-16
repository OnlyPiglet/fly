package runtimetools

import (
	"fmt"
	"runtime"
	"runtime/debug"
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

// Frame 表示一个堆栈帧
type Frame struct {
	File     string `json:"file"`     // 文件路径
	Line     int    `json:"line"`     // 行号
	Function string `json:"function"` // 函数名
}

// String 返回单个堆栈帧的字符串表示
func (f Frame) String() string {
	return fmt.Sprintf("%s:%d - %s", f.File, f.Line, f.Function)
}

// Frames 表示堆栈帧数组
type Frames []Frame

// GetStackFrames 获取结构化的堆栈帧数组（从上到下）
// skip: 跳过的层数，通常为1（跳过GetStackFrames本身）
// maxFrames: 最大获取的帧数，0表示不限制
func GetStackFrames(skip int, maxFrames int) Frames {
	if maxFrames <= 0 {
		maxFrames = 32 // 默认最多32层
	}

	pc := make([]uintptr, maxFrames)
	n := runtime.Callers(skip+1, pc) // +1 跳过GetStackFrames本身
	if n == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pc[:n])
	var stackFrames []Frame

	for {
		frame, more := frames.Next()
		// 可以选择性过滤某些包的调用
		if !strings.Contains(frame.Function, "runtime.") {
			stackFrames = append(stackFrames, Frame{
				File:     frame.File,
				Line:     frame.Line,
				Function: frame.Function,
			})
		}
		if !more {
			break
		}
	}

	return stackFrames
}

// GetStackFramesSimple 获取简化的堆栈帧数组（只包含文件名，不包含完整路径）
func GetStackFramesSimple(skip int, maxFrames int) Frames {
	frames := GetStackFrames(skip+1, maxFrames) // +1 跳过GetStackFramesSimple本身

	// 简化文件路径，只保留文件名
	for i := range frames {
		if lastSlash := strings.LastIndex(frames[i].File, "/"); lastSlash != -1 {
			frames[i].File = frames[i].File[lastSlash+1:]
		}
	}

	return frames
}

// GetStackFramesFiltered 获取过滤后的堆栈帧数组
// excludePatterns: 要排除的函数名模式
func GetStackFramesFiltered(skip int, maxFrames int, excludePatterns []string) Frames {
	if maxFrames <= 0 {
		maxFrames = 32
	}

	pc := make([]uintptr, maxFrames)
	n := runtime.Callers(skip+1, pc)
	if n == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pc[:n])
	var stackFrames Frames

	for {
		frame, more := frames.Next()

		// 检查是否需要排除这个函数
		shouldExclude := false
		for _, pattern := range excludePatterns {
			if strings.Contains(frame.Function, pattern) {
				shouldExclude = true
				break
			}
		}

		if !shouldExclude {
			// 简化文件路径
			file := frame.File
			if lastSlash := strings.LastIndex(file, "/"); lastSlash != -1 {
				file = file[lastSlash+1:]
			}

			stackFrames = append(stackFrames, Frame{
				File:     file,
				Line:     frame.Line,
				Function: frame.Function,
			})
		}

		if !more {
			break
		}
	}

	return stackFrames
}

// GetCurrentFrame 获取当前调用者的帧信息
func GetCurrentFrame() *Frame {
	pc := make([]uintptr, 2)
	n := runtime.Callers(2, pc) // 跳过GetCurrentFrame和Callers
	if n == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pc[:1])
	frame, _ := frames.Next()

	file := frame.File
	if lastSlash := strings.LastIndex(file, "/"); lastSlash != -1 {
		file = file[lastSlash+1:]
	}

	return &Frame{
		File:     file,
		Line:     frame.Line,
		Function: frame.Function,
	}
}

// FramesToString 将堆栈帧数组格式化为字符串，每多一层前缀多两个空格
func (frames Frames) String() string {
	if len(frames) == 0 {
		return ""
	}

	var result strings.Builder

	for i, frame := range frames {
		// 计算前缀空格数量：每层2个空格
		prefix := strings.Repeat("  ", i)

		// 使用 Frame 的 String 方法
		result.WriteString(prefix + frame.String())

		// 除了最后一行，都添加换行符
		if i < len(frames)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
