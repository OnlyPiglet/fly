package runtimetools

import (
	"fmt"
	"testing"
)

func TestStack(t *testing.T) {
	stack1()
}

func stack1() {
	stack2()
}

func stack2() {
	stack3()
}

func stack3() {
	stack4()
}

func stack4() {
	frames := GetStackFrames(1, 10)
	fmt.Println(frames)
}
