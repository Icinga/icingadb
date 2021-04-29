package structify

import (
	"encoding"
	"fmt"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/pkg/errors"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

// structBranch represents either a leaf or a subTree.
type structBranch struct {
	// field specifies the struct field index.
	field int
	// leaf specifies the map key to parse the struct field from.
	leaf string
	// subTree specifies the struct field's inner tree.
	subTree []structBranch
}

type MapStructifier = func(map[string]interface{}) (interface{}, error)

// MakeMapStructifier builds a function which parses a map's string values into a new struct of type t
// and returns a pointer to it. tag specifies which tag connects struct fields to map keys.
// MakeMapStructifier panics if it detects an unsupported type (suitable for usage in init() or global vars).
func MakeMapStructifier(t reflect.Type, tag string) MapStructifier {
	tree := buildStructTree(t, tag)

	return func(kv map[string]interface{}) (interface{}, error) {
		vPtr := reflect.New(t)
		ptr := vPtr.Interface()

		if initer, ok := ptr.(contracts.Initer); ok {
			initer.Init()
		}

		vPtrElem := vPtr.Elem()
		return ptr, structifyMapByTree(kv, tree, vPtrElem, vPtrElem, new([]int))
	}
}

// buildStructTree assembles a tree which represents the struct t based on tag.
func buildStructTree(t reflect.Type, tag string) []structBranch {
	var tree []structBranch
	numFields := t.NumField()

	for i := 0; i < numFields; i++ {
		if field := t.Field(i); field.PkgPath == "" {
			switch tagValue := field.Tag.Get(tag); tagValue {
			case "", "-":
			case ",inline":
				if subTree := buildStructTree(field.Type, tag); subTree != nil {
					tree = append(tree, structBranch{i, "", subTree})
				}
			default:
				// If parseString doesn't support *T, it'll panic.
				_ = parseString("", reflect.New(field.Type).Interface())

				tree = append(tree, structBranch{i, tagValue, nil})
			}
		}
	}

	return tree
}

// structifyMapByTree parses src's string values into the struct dest according to tree's specification.
func structifyMapByTree(src map[string]interface{}, tree []structBranch, dest, root reflect.Value, stack *[]int) error {
	*stack = append(*stack, 0)
	defer func() {
		*stack = (*stack)[:len(*stack)-1]
	}()

	for _, branch := range tree {
		(*stack)[len(*stack)-1] = branch.field

		if branch.subTree == nil {
			if v, ok := src[branch.leaf]; ok {
				if vs, ok := v.(string); ok {
					if err := parseString(vs, dest.Field(branch.field).Addr().Interface()); err != nil {
						rt := root.Type()
						typ := rt
						var path []string

						for _, i := range *stack {
							f := typ.Field(i)
							path = append(path, f.Name)
							typ = f.Type
						}

						return errors.Wrap(err, fmt.Sprintf(
							"can't parse %s into the %s %s#%s: %s", branch.leaf,
							typ.Name(), rt.Name(), strings.Join(path, "."), vs,
						))
					}
				}
			}
		} else if err := structifyMapByTree(src, branch.subTree, dest.Field(branch.field), root, stack); err != nil {
			return err
		}
	}

	return nil
}

// parseString parses src into *dest.
func parseString(src string, dest interface{}) error {
	switch ptr := dest.(type) {
	case encoding.TextUnmarshaler:
		return ptr.UnmarshalText([]byte(src))
	case *string:
		*ptr = src
		return nil
	case **string:
		*ptr = &src
		return nil
	case *uint8:
		i, err := strconv.ParseUint(src, 10, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = uint8(i)
		}
		return err
	case *uint16:
		i, err := strconv.ParseUint(src, 10, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = uint16(i)
		}
		return err
	case *uint32:
		i, err := strconv.ParseUint(src, 10, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = uint32(i)
		}
		return err
	case *uint64:
		i, err := strconv.ParseUint(src, 10, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = i
		}
		return err
	case *int8:
		i, err := strconv.ParseInt(src, 10, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = int8(i)
		}
		return err
	case *int16:
		i, err := strconv.ParseInt(src, 10, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = int16(i)
		}
		return err
	case *int32:
		i, err := strconv.ParseInt(src, 10, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = int32(i)
		}
		return err
	case *int64:
		i, err := strconv.ParseInt(src, 10, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = i
		}
		return err
	case *float32:
		f, err := strconv.ParseFloat(src, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = float32(f)
		}
		return err
	case *float64:
		f, err := strconv.ParseFloat(src, int(unsafe.Sizeof(*ptr)*8))
		if err == nil {
			*ptr = f
		}
		return err
	default:
		panic(fmt.Sprintf("unsupported type: %T", dest))
	}
}
