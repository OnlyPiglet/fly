package map_tools

import "testing"

func TestExistedInMap(t *testing.T) {

	t.Run("stingStringExitsedInMap", func(t *testing.T) {

		m := make(map[string]string)
		m["1"] = "1"
		m["2"] = "2"

		println(ExistedInMap("1", m))
		println(ExistedInMap("3", m))

	})

}
