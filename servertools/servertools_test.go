package servertools

import (
	"fmt"
	"testing"
)

func TestServerInfo(t *testing.T) {
	in := ServerInfo()
	println(fmt.Sprintf("%+v", in))
}
