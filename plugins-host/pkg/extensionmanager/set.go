package extensionmanager

type Set[T comparable] struct {
	m map[T]struct{}
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{m: make(map[T]struct{})}
}

func NewSetFromSlice[T comparable](tt []T) *Set[T] {
	s := &Set[T]{m: make(map[T]struct{})}
	for _, t := range tt {
		s.m[t] = struct{}{}
	}
	return s
}

func (s *Set[T]) Add(t T) {
	s.m[t] = struct{}{}
}

func (s *Set[T]) AddAll(st *Set[T]) {
	for t := range st.m {
		s.m[t] = struct{}{}
	}
}

func (s *Set[T]) Remove(t T) {
	delete(s.m, t)
}

func (s *Set[T]) Contains(t T) bool {
	_, ok := s.m[t]
	return ok
}

func (s *Set[T]) Clear() {
	s.m = make(map[T]struct{})
}

func (s *Set[T]) Values() []T {
	var res []T
	for v := range s.m {
		res = append(res, v)
	}
	return res
}

func (s *Set[T]) Len() int {
	return len(s.m)
}
