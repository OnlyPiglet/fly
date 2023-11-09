package chan_tools

import "testing"

func TestChanTool(t *testing.T) {

	t.Run("safe close chantools", func(t *testing.T) {

		ch := make(chan int, 1)
		tools := NewChanTools(&ch)

		tools.SafeCloseCh(ch)
		tools.SafeCloseCh(ch)
		tools.SafeCloseCh(ch)

	})

}
