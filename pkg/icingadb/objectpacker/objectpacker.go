package objectpacker

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"sort"
)

// PackAny packs any JSON-encodable value (ex. structs, also ignores interfaces like encoding.TextMarshaler)
// to a BSON-similar format suitable for consistent hashing. Spec:
// https://github.com/Icinga/icinga2/blob/2cb995e/lib/base/object-packer.cpp#L222-L231
func PackAny(in interface{}, out io.Writer) error {
	return packValue(reflect.ValueOf(in), out)
}

// packValue does the actual job of packAny and just exists for recursion w/o unneccessary reflect.ValueOf calls.
func packValue(in reflect.Value, out io.Writer) error {
	switch in.Kind() {
	case reflect.Invalid: // nil
		_, err := out.Write([]byte{0})
		return err
	case reflect.Bool:
		if in.Bool() {
			_, err := out.Write([]byte{2})
			return err
		} else {
			_, err := out.Write([]byte{1})
			return err
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return packFloat64(float64(in.Int()), out)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return packFloat64(float64(in.Uint()), out)
	case reflect.Float32, reflect.Float64:
		return packFloat64(in.Float(), out)
	case reflect.Array, reflect.Slice:
		if _, err := out.Write([]byte{5}); err != nil {
			return err
		}

		l := in.Len()
		if err := binary.Write(out, binary.BigEndian, uint64(l)); err != nil {
			return err
		}

		for i := 0; i < l; i++ {
			if err := packValue(in.Index(i), out); err != nil {
				return err
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

		if _, err := out.Write([]byte{6}); err != nil {
			return err
		}

		l := in.Len()
		if err := binary.Write(out, binary.BigEndian, uint64(l)); err != nil {
			return err
		}

		sorted := make([]kv, 0, l)

		{
			iter := in.MapRange()
			for iter.Next() {
				// Disallow (panic) some types in map keys (recursively), too
				_ = packValue(iter.Key(), ioutil.Discard)

				sorted = append(sorted, kv{[]byte(fmt.Sprint(iter.Key().Interface())), iter.Value()})
			}
		}

		sort.Slice(sorted, func(i, j int) bool { return bytes.Compare(sorted[i].key, sorted[j].key) < 0 })

		for _, kv := range sorted {
			if err := binary.Write(out, binary.BigEndian, uint64(len(kv.key))); err != nil {
				return err
			}

			if _, err := out.Write(kv.key); err != nil {
				return err
			}

			if err := packValue(kv.value, out); err != nil {
				return err
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
		if _, err := out.Write([]byte{4}); err != nil {
			return err
		}

		b := []byte(in.String())
		if err := binary.Write(out, binary.BigEndian, uint64(len(b))); err != nil {
			return err
		}

		_, err := out.Write(b)
		return err
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
