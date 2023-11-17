package httptools

import "testing"

type Person struct {
	Name  string  `form:"name"`
	Age   int     `form:"age"`
	Email string  `form:"email"`
	Male  bool    `form:"male"`
	Score float32 `form:"score"`
	Mi    int8    `form:"mi"`
	Umi   uint8   `form:"umi"`
}

func TestStruct2URLValues(t *testing.T) {

	person := Person{
		Name:  "jackwu",
		Age:   100,
		Email: "jackwuchenghao4@gmail.com",
		Male:  true,
		Score: 23.3,
		Mi:    -6,
		Umi:   8,
	}

	values, err := Struct2URLValues[Person](person)

	if err != nil {
		println(err.Error())
		return
	}

	println(values.Encode())

}
