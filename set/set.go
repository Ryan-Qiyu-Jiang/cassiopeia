package set

import (
	"fmt"
	"strings"
)

// OrderedSet is a ordered set, with arr as set, O(n) insert select cus of copy
type OrderedSet struct {
	Arr []string
}

// Remove removes e from ordered set
func (s *OrderedSet) Remove(e string) {
	if len(s.Arr) == 0 {
		return
	} else if strings.Compare(e, s.Arr[0]) < 0 {
		return
	} else if strings.Compare(e, s.Arr[len(s.Arr)-1]) > 0 {
		return
	} else {
		h := len(s.Arr) - 1
		l := 0
		if e == s.Arr[h] {
			s.Arr = s.Arr[:h]
			return
		}
		if e == s.Arr[l] {
			s.Arr = s.Arr[1:]
			return
		}
		for h > l+1 {
			m := int((h + l) / 2)
			mVal := s.Arr[m]
			if mVal == e {
				copy(s.Arr[m:], s.Arr[m+1:])
				s.Arr = s.Arr[:h]
				return
			} else if strings.Compare(e, mVal) < 0 {
				h = m
			} else {
				l = m
			}
		}
	}
}

// Find returns index in set, or -1
func (s *OrderedSet) Find(e string) int {
	if len(s.Arr) == 0 {
		return -1
	} else if strings.Compare(e, s.Arr[0]) < 0 {
		return -1
	} else if strings.Compare(e, s.Arr[len(s.Arr)-1]) > 0 {
		return -1
	} else {
		h := len(s.Arr) - 1
		l := 0
		if e == s.Arr[h] {
			return h
		}
		if e == s.Arr[l] {
			return l
		}
		for h > l+1 {
			m := int((h + l) / 2)
			mVal := s.Arr[m]
			if mVal == e {
				return m
			} else if strings.Compare(e, mVal) < 0 {
				h = m
			} else {
				l = m
			}
		}
	}
	return -1
}

// Add e to ordered set
func (s *OrderedSet) Add(e string) {
	if len(s.Arr) == 0 {
		s.Arr = []string{e}
	} else if strings.Compare(e, s.Arr[0]) < 0 {
		s.Arr = append([]string{e}, s.Arr...)
	} else if strings.Compare(e, s.Arr[len(s.Arr)-1]) > 0 {
		s.Arr = append(s.Arr, e)
	} else {
		h := len(s.Arr) - 1
		l := 0
		if e == s.Arr[h] || e == s.Arr[l] {
			return
		}
		for h > l+1 {
			m := int((h + l) / 2)
			mVal := s.Arr[m]
			if mVal == e {
				return
			} else if strings.Compare(e, mVal) < 0 {
				h = m
			} else {
				l = m
			}
		}
		s.Arr = append(s.Arr, "")
		copy(s.Arr[h+1:], s.Arr[h:])
		s.Arr[h] = e
	}
}

// NewSet is a orderedset singleton factory
func NewSet() *OrderedSet {
	return &OrderedSet{Arr: []string{}}
}

func main() {
	set := NewSet()
	fmt.Println(set.Arr)
	set.Add("ryan")
	fmt.Println(set.Arr)
	set.Add("a")
	fmt.Println(set.Arr)
	set.Add("abby")
	fmt.Println(set.Arr)
	set.Add("thomas")
	fmt.Println(set.Arr)
	set.Remove("ryan")
	fmt.Println(set.Arr)
	set.Remove("a")
	fmt.Println(set.Arr)
	set.Add("ryan")
	fmt.Println(set.Arr)
	set.Remove("ryan")
	fmt.Println(set.Arr)
}
