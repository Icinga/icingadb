package flatten

import (
	"strconv"
)

// Flatten creates flat, one-dimensional maps from arbitrarily nested values, e.g. JSON.
func Flatten(value interface{}, prefix string) map[string]interface{} {
	var flatten func(string, interface{})
	flattened := make(map[string]interface{})

	flatten = func(key string, value interface{}) {
		switch value := value.(type) {
		case map[string]interface{}:
			for k, v := range value {
				flatten(key+"."+k, v)
			}
		case []interface{}:
			for i, v := range value {
				flatten(key+"["+strconv.Itoa(i)+"]", v)
			}
		default:
			flattened[key] = value
		}
	}

	flatten(prefix, value)

	return flattened
}
