package slice_tools

func ContainBy[T any](slice []T, dest T, equal func(s T, d T) bool) bool {
	for _, sitem := range slice {
		if equal(sitem, dest) {
			return true
		}
	}

	return false
}

func LimitOffset[T any](slice []T, page int, pageSize int) []T {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	mr := len(slice)
	l := (page - 1) * pageSize
	r := page * pageSize
	if l > mr {
		l = mr
	}
	if r > mr {
		r = mr
	}
	return slice[l:r]
}

func Unique[T any](src []T, equal func(s T, d T) bool) []T {
	dst := make([]T, 0)
	for _, t := range src {
		if ContainBy(dst, t, equal) {
			continue
		} else {
			dst = append(dst, t)
		}
	}
	return dst
}

func FindBy[T any](slice []T, dest T, equal func(s T, d T) bool) *T {
	for _, sitem := range slice {
		if equal(sitem, dest) {
			return &sitem
		}
	}

	return nil
}

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
