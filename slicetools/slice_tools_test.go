package slice_tools

import (
	"fmt"
	"log"
	"testing"
)

func TestName(t *testing.T) {
	t.Run("dst which not in src", func(t *testing.T) {
		src := []int{1, 2, 3}
		dst := []int{2, 3, 4, 5}
		inDst := InDstNotInSrc(src, dst)
		for _, i := range inDst {
			println(i)
		}
	})
}

func TestUniqueSlice(t *testing.T) {

	t.Run("testUniqueIntSlice", func(t *testing.T) {
		a := []int{1, 1, 2, 2, 4}

		slice := UniqueSlice(a)

		for _, v := range slice {
			println(v)
		}
	})

	t.Run("testUniqueStringSlice", func(t *testing.T) {
		a := []string{"1", "3", "2", "2", "4"}

		slice := UniqueSlice(a)

		for _, v := range slice {
			println(v)
		}
	})

}

func TestRemoveElem(t *testing.T) {

	a := []string{"a", "v", "c", "s", "a", "12"}

	println(fmt.Sprintf("%+v", a))

	a = RemoveElem[string](a, "12")

	println(fmt.Sprintf("%+v", a))

}

func TestLimitOffset(t *testing.T) {
	a := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}

	//page = 1 ,pageSize = 2
	log.Printf("%v", LimitOffset[int](a, 1, 2))
	log.Printf("%v", a[0:2])

	//page = 2 ,pageSize = 2
	log.Printf("%v", LimitOffset[int](a, 2, 2))

	log.Printf("%v", a[2:4])

	//page = 2 ,pageSize = 3
	log.Printf("%v", LimitOffset[int](a, 2, 3))
	log.Printf("%v", a[3:6])

	log.Printf("%v", LimitOffset[int](a, 1, 10))

}
