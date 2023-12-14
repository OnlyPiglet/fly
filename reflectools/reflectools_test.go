package reflectools

import "testing"

func TestGetTypeOfInter(t *testing.T) {
	a := 2
	println(GetInterfaceType(a))
	b := uint(2)
	println(GetInterfaceType(b))
	c := "a"
	println(GetInterfaceType(c))
	d := 2.1
	println(GetInterfaceType(d))
}
