package utils

import (
	"fmt"
	"reflect"
)

// AnySliceToInterfaceSlice takes a slice of type []T for any T and returns a slice of type []interface{} containing
// the same elements, somewhat like casting []T to []interface{}.
func AnySliceToInterfaceSlice(in interface{}) []interface{} {
	v := reflect.ValueOf(in)
	if v.Kind() != reflect.Slice {
		panic(fmt.Errorf("AnySliceToInterfaceSlice() called on %T instead of a slice type", in))
	}

	out := make([]interface{}, v.Len())
	for i := 0; i < v.Len(); i++ {
		out[i] = v.Index(i).Interface()
	}
	return out
}

func SliceSubsets(in ...string) [][]string {
	result := make([][]string, 0, 1<<len(in))

	for bitset := 0; bitset < (1 << len(in)); bitset++ {
		var subset []string
		for i := 0; i < len(in); i++ {
			if bitset&(1<<i) != 0 {
				subset = append(subset, in[i])
			}
		}
		result = append(result, subset)
	}

	return result
}
