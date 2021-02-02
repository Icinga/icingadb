package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
	"sort"
)

// nullWriter discards written data.
type nullWriter struct{}

var _ io.Writer = nullWriter{}

// Write never fails.
func (nullWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

// hashStr hashes s via SHA1.
func hashStr(s string) []byte {
	hash := sha1.New()
	_, _ = fmt.Fprint(hash, s)
	return hash.Sum(nil)
}

// hashAny combines packAny and SHA1 hashing.
func hashAny(in interface{}) []byte {
	hash := sha1.New()
	_ = packAny(in, hash)
	return hash.Sum(nil)
}

// calcObjectId calculates the ID of the config object named name1 for Icinga DB.
func calcObjectId(name1 string) interface{} {
	if name1 == "" {
		return nil
	}

	return hashAny([2]string{icingaEnv.value, name1})
}

// calcServiceId calculates the ID of the service name2 of the host name1 for Icinga DB.
func calcServiceId(name1, name2 string) interface{} {
	if name2 == "" {
		return nil
	}

	return hashAny([2]string{icingaEnv.value, name1 + "!" + name2})
}

// packAny packs any JSON-encodable value (ex. structs, also ignores interfaces like encoding.TextMarshaler)
// to a BSON-similar format suitable for consistent hashing. Spec:
// https://github.com/Icinga/icinga2/blob/2cb995e/lib/base/object-packer.cpp#L222-L231
func packAny(in interface{}, out io.Writer) error {
	return packValue(reflect.ValueOf(in), out)
}

// packValue does the actual job of packAny and just exist for recursion w/o unneccessary reflect.ValueOf calls.
func packValue(in reflect.Value, out io.Writer) error {
	switch in.Kind() {
	case reflect.Invalid: // nil
		_, errWr := out.Write([]byte{0})
		return errWr
	case reflect.Bool:
		if in.Bool() {
			_, errWr := out.Write([]byte{2})
			return errWr
		} else {
			_, errWr := out.Write([]byte{1})
			return errWr
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return packFloat64(float64(in.Int()), out)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return packFloat64(float64(in.Uint()), out)
	case reflect.Float32, reflect.Float64:
		return packFloat64(in.Float(), out)
	case reflect.Array, reflect.Slice:
		if _, errWr := out.Write([]byte{5}); errWr != nil {
			return errWr
		}

		l := in.Len()
		if errBW := binary.Write(out, binary.BigEndian, uint64(l)); errBW != nil {
			return errBW
		}

		for i := 0; i < l; i++ {
			if errPV := packValue(in.Index(i), out); errPV != nil {
				return errPV
			}
		}

		return nil
	case reflect.Interface:
		return packValue(in.Elem(), out)
	case reflect.Map:
		type kv struct {
			key   []byte
			value reflect.Value
		}

		if _, errWr := out.Write([]byte{6}); errWr != nil {
			return errWr
		}

		l := in.Len()
		if errBW := binary.Write(out, binary.BigEndian, uint64(l)); errBW != nil {
			return errBW
		}

		sorted := make([]kv, 0, l)

		{
			iter := in.MapRange()
			for iter.Next() {
				// Disallow (panic) some types in map keys (recursively), too
				_ = packValue(iter.Key(), nullWriter{})

				sorted = append(sorted, kv{[]byte(fmt.Sprint(iter.Key().Interface())), iter.Value()})
			}
		}

		sort.Slice(sorted, func(i, j int) bool { return bytes.Compare(sorted[i].key, sorted[j].key) < 0 })

		for _, kv := range sorted {
			if errBW := binary.Write(out, binary.BigEndian, uint64(len(kv.key))); errBW != nil {
				return errBW
			}

			if _, errWr := out.Write(kv.key); errWr != nil {
				return errWr
			}

			if errPV := packValue(kv.value, out); errPV != nil {
				return errPV
			}
		}

		return nil
	case reflect.Ptr:
		if in.IsNil() {
			return packValue(reflect.Value{}, out)
		} else {
			return packValue(in.Elem(), out)
		}
	case reflect.String:
		if _, errWr := out.Write([]byte{4}); errWr != nil {
			return errWr
		}

		b := []byte(in.String())
		if errBW := binary.Write(out, binary.BigEndian, uint64(len(b))); errBW != nil {
			return errBW
		}

		_, errWr := out.Write(b)
		return errWr
	default:
		panic("bad type: " + in.Kind().String())
	}
}

// packFloat64 deduplicates float packing of multiple locations in packValue.
func packFloat64(in float64, out io.Writer) error {
	if _, errWr := out.Write([]byte{3}); errWr != nil {
		return errWr
	}

	return binary.Write(out, binary.BigEndian, in)
}
