package slice_tools

func UniqueSlice[T comparable](in []T) []T {

	if in == nil || len(in) == 0 {
		return []T{}
	}

	o := make(map[T]bool, 0)
	out := make([]T, 0)

	for _, n := range in {
		if !o[n] {
			o[n] = true
			out = append(out, n)
		}
		continue
	}

	return out
}

func ContainsElem[T comparable](dst []T, com T) bool {
	for _, t := range dst {
		if t == com {
			return true
		}
	}
	return false
}

func InDstNotInSrc[T comparable](src, dst []T) []T {
	m := make(map[T]bool)

	for _, item := range src {
		m[item] = true
	}

	diff := make([]T, 0)

	for _, item := range dst {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}

	return diff
}

func RemoveElem[T comparable](dst []T, com T) []T {
	m := make([]T, 0)

	for i, _ := range dst {
		if dst[i] == com {
			continue
		}
		m = append(m, dst[i])
	}

	return m
}
