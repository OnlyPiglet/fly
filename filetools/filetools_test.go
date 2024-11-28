package filetools

import "testing"

func TestMaxKept(t *testing.T) {
	err := RecentFileMaxKept("./test", 2)
	if err != nil {
		t.Fatal(err)
	}
}
