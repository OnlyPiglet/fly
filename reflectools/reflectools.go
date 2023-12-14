package reflectools

import "reflect"

func GetInterfaceType(a any) string {
	return reflect.TypeOf(a).String()
}
