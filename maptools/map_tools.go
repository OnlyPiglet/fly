package maptools

func ExistedInMap[K comparable, V any](k K, m map[K]V) bool {

	for mk, _ := range m {
		if mk == k {
			return true
		}
	}
	return false
}
