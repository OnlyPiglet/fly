package slice

import "testing"

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
