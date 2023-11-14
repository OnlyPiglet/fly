package jsontools

import (
	"fmt"
	"testing"
)

type User struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestJsonMash(t *testing.T) {
	user := User{
		Name: "tt",
		Age:  12,
	}
	_ = t.Run("userMash", func(t *testing.T) {
		mash, _ := JsonMash(user)
		println(mash)
	})
	_ = t.Run("userUnMash", func(t *testing.T) {
		var user User
		userStr := "{\"name\":\"tt\",\"age\":12}"
		_ = JsonUnMash(userStr, &user)
		println(fmt.Sprintf("%v", user))
	})

}
