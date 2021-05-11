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

// MustPackAny calls PackAny using in and panics if there was an error.
func MustPackAny(in ...interface{}) []byte {
	var buf bytes.Buffer

	if err := PackAny(in, &buf); err != nil {
		panic(err)
	}

	return buf.Bytes()
}

// PackAny packs any JSON-encodable value (ex. structs, also ignores interfaces like encoding.TextMarshaler)
// to a BSON-similar format suitable for consistent hashing. Spec:
//
// PackAny(nil)            => 0x0
// PackAny(false)          => 0x1
// PackAny(true)           => 0x2
// PackAny(float64(42))    => 0x3 ieee754_binary64_bigendian(42)
// PackAny("exämple")      => 0x4 uint64_bigendian(len([]byte("exämple"))) []byte("exämple")
// PackAny([]uint8{0x42})  => 0x4 uint64_bigendian(len([]uint8{0x42})) []uint8{0x42}
// PackAny([1]uint8{0x42}) => 0x4 uint64_bigendian(len([1]uint8{0x42})) [1]uint8{0x42}
// PackAny([]T{x,y})       => 0x5 uint64_bigendian(len([]T{x,y})) PackAny(x) PackAny(y)
// PackAny(map[K]V{x:y})   => 0x6 uint64_bigendian(len(map[K]V{x:y})) len(map_key(x)) map_key(x) PackAny(y)
// PackAny((*T)(nil))      => 0x0
// PackAny((*T)(0x42))     => PackAny(*(*T)(0x42))
// PackAny(x)              => panic()
//
// map_key([1]uint8{0x42}) => [1]uint8{0x42}
// map_key(x)              => []byte(fmt.Sprint(x))
func PackAny(in interface{}, out io.Writer) error {
	return packValue(reflect.ValueOf(in), out)
}

var tByte = reflect.TypeOf(byte(0))
var tBytes = reflect.TypeOf([]uint8(nil))

// packValue does the actual job of packAny and just exists for recursion w/o unneccessary reflect.ValueOf calls.
func packValue(in reflect.Value, out io.Writer) error {
	switch kind := in.Kind(); kind {
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
	case reflect.Float64:
		if _, err := out.Write([]byte{3}); err != nil {
			return err
		}

		return binary.Write(out, binary.BigEndian, in.Float())
	case reflect.Array, reflect.Slice:
		if typ := in.Type(); typ.Elem() == tByte {
			if kind == reflect.Array {
				if !in.CanAddr() {
					vNewElem := reflect.New(typ).Elem()
					vNewElem.Set(in)
					in = vNewElem
				}

				in = in.Slice(0, in.Len())
			}

			// Pack []byte as string, not array of numbers.
			return packString(in.Convert(tBytes). // Support types.Binary
								Interface().([]uint8), out)
		}

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

		// If there aren't any values to pack, ...
		if l < 1 {
			// ... create one and pack it - panics on disallowed type.
			_ = packValue(reflect.Zero(in.Type().Elem()), ioutil.Discard)
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
				var packedKey []byte
				if key := iter.Key(); key.Kind() == reflect.Array {
					if typ := key.Type(); typ.Elem() == tByte {
						if !key.CanAddr() {
							vNewElem := reflect.New(typ).Elem()
							vNewElem.Set(key)
							key = vNewElem
						}

						packedKey = key.Slice(0, key.Len()).Interface().([]byte)
					} else {
						// Not just stringify the key (below), but also pack it (here) - panics on disallowed type.
						_ = packValue(iter.Key(), ioutil.Discard)

						packedKey = []byte(fmt.Sprint(key.Interface()))
					}
				} else {
					// Not just stringify the key (below), but also pack it (here) - panics on disallowed type.
					_ = packValue(iter.Key(), ioutil.Discard)

					packedKey = []byte(fmt.Sprint(key.Interface()))
				}

				sorted = append(sorted, kv{packedKey, iter.Value()})
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

		// If there aren't any key-value pairs to pack, ...
		if l < 1 {
			typ := in.Type()

			// ... create one and pack it - panics on disallowed type.
			_ = packValue(reflect.Zero(typ.Key()), ioutil.Discard)
			_ = packValue(reflect.Zero(typ.Elem()), ioutil.Discard)
		}

		return nil
	case reflect.Ptr:
		if in.IsNil() {
			err := packValue(reflect.Value{}, out)

			// Create a fictive referenced value and pack it - panics on disallowed type.
			_ = packValue(reflect.Zero(in.Type().Elem()), ioutil.Discard)

			return err
		} else {
			return packValue(in.Elem(), out)
		}
	case reflect.String:
		return packString([]byte(in.String()), out)
	default:
		panic("bad type: " + in.Kind().String())
	}
}

// packString deduplicates string packing of multiple locations in packValue.
func packString(in []byte, out io.Writer) error {
	if _, err := out.Write([]byte{4}); err != nil {
		return err
	}

	if err := binary.Write(out, binary.BigEndian, uint64(len(in))); err != nil {
		return err
	}

	_, err := out.Write(in)
	return err
}
