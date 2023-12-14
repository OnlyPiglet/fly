package reflectools

import "reflect"

//reflect thanks for https://www.cnblogs.com/jiujuan/p/17142703.html

func GetInterfaceType(a any) string {
	return reflect.TypeOf(a).String()
}
