package utils

func DummyList[T any]() []T {
	var dummy [0]T
	return dummy[:0]
}

func ReplaceNilByDummy[T any](list []T) []T {
	if list == nil {
		return DummyList[T]()
	}
	return list
}

func ListToSet[T comparable](list []T) map[T]bool {
	set := make(map[T]bool, len(list))
	ListIntoSet(list, set)
	return set
}

func ListIntoSet[T comparable](list []T, set map[T]bool) {
	for _, one := range list {
		set[one] = true
	}
}
