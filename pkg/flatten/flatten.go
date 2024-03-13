package flatten

import (
	"fmt"
	"github.com/icinga/icingadb/pkg/types"
	"strconv"
)

// Flatten creates flat, one-dimensional maps from arbitrarily nested values, e.g. JSON.
func Flatten(value interface{}, prefix string) map[string]types.String {
	var flatten func(string, interface{})
	flattened := make(map[string]types.String)

	flatten = func(key string, value interface{}) {
		switch value := value.(type) {
		case map[string]interface{}:
			if len(value) == 0 {
				flattened[key] = types.String{}
				break
			}

			for k, v := range value {
				flatten(key+"."+k, v)
			}
		case []interface{}:
			if len(value) == 0 {
				flattened[key] = types.String{}
				break
			}

			for i, v := range value {
				flatten(key+"["+strconv.Itoa(i)+"]", v)
			}
		default:
			val := "null"
			if value != nil {
				val = fmt.Sprintf("%v", value)
			}
			flattened[key] = types.MakeString(val)
		}
	}

	flatten(prefix, value)

	return flattened
}
