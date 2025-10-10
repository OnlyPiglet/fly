package processtools

import (
	"testing"
	"time"
)

func TestStartSingleProcess(t *testing.T) {
	err, c := StartSingleProcess("test.lock")
	if err != nil {
		t.Fatal(err)
	}
	err2, _ := StartSingleProcess("test.lock")
	if err2 != nil {
		println(err2.Error())
	}
	time.Sleep(10 * time.Second)
	close(c)
	err3, _ := StartSingleProcess("test.lock")
	if err3 != nil {
		t.Error(err3)
	}
}
