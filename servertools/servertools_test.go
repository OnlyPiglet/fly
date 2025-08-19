package servertools

import (
	"fmt"
	"testing"
)

func TestServerInfo(t *testing.T) {
	in := ServerInfo()
	println(fmt.Sprintf("%+v", in))
}

func TestGetIPv4ByInterfaceName(t *testing.T) {
	iface, err := GetIPv4ByInterfaceName("en0")
	if err != nil {
		t.Fatal(err)
	}
	println(fmt.Sprintf("%+v", iface))
}
