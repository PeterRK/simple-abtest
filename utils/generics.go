package utils

// DummyList returns a non-nil empty slice for type T.
func DummyList[T any]() []T {
	var dummy [0]T
	return dummy[:0]
}

// ReplaceNilByDummy converts a nil slice to a non-nil empty slice.
func ReplaceNilByDummy[T any](list []T) []T {
	if list == nil {
		return DummyList[T]()
	}
	return list
}

// ListToSet builds a map-based set from list with all elements set to true.
func ListToSet[T comparable](list []T) map[T]bool {
	set := make(map[T]bool, len(list))
	ListIntoSet(list, set)
	return set
}

// ListIntoSet inserts all elements of list into set.
func ListIntoSet[T comparable](list []T, set map[T]bool) {
	for _, one := range list {
		set[one] = true
	}
}
