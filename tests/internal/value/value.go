package value

import (
	"fmt"
	"reflect"
)

func ToIcinga2Config(value interface{}) string {
	if value == nil {
		return "null"
	}

	refVal := reflect.ValueOf(value)
	switch refVal.Kind() {
	case reflect.Slice:
		vs := ""
		for i := 0; i < refVal.Len(); i++ {
			vs += ToIcinga2Config(refVal.Index(i).Interface()) + ","
		}
		return "[" + vs + "]"
	case reflect.Map:
		kvs := ""
		iter := refVal.MapRange()
		for iter.Next() {
			kvs += ToIcinga2Config(iter.Key().Interface()) + "=" + ToIcinga2Config(iter.Value().Interface()) + ","
		}
		return "{" + kvs + "}"
	}

	switch v := value.(type) {
	case interface{ Icinga2ConfigValue() string }:
		return v.Icinga2ConfigValue()
	case string:
		// TODO(jb): probably not perfect quoting, but good enough for now
		return fmt.Sprintf("%q", v)
	case *string:
		if v != nil {
			return ToIcinga2Config(*v)
		} else {
			return "null"
		}
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", v)
	default:
		panic(fmt.Errorf("ToIcinga2Config called on unknown type %T", value))
	}
}

func ToIcinga2Api(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	refVal := reflect.ValueOf(value)
	switch refVal.Kind() {
	case reflect.Slice:
		r := make([]interface{}, refVal.Len())
		for i := range r {
			r[i] = ToIcinga2Api(refVal.Index(i).Interface())
		}
		return r
	case reflect.Map:
		r := make(map[string]interface{})
		iter := refVal.MapRange()
		for iter.Next() {
			// TODO: perform a better check than the type assertion
			r[ToIcinga2Api(iter.Key().Interface()).(string)] = ToIcinga2Api(iter.Value().Interface())
		}
		return r
	}

	switch v := value.(type) {
	case interface{ Icinga2ApiValue() interface{} }:
		return v.Icinga2ApiValue()
	case string, []string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return v
	case *string:
		if v != nil {
			return ToIcinga2Api(*v)
		} else {
			return nil
		}
	default:
		panic(fmt.Errorf("ToIcinga2Api called on unknown type %T", value))
	}
}

func ToIcingaDb(value interface{}) interface{} {
	switch v := value.(type) {
	case interface{ IcingaDbValue() interface{} }:
		return v.IcingaDbValue()
	case bool:
		if v {
			return "y"
		} else {
			return "n"
		}
	case string, *string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return v
	default:
		panic(fmt.Errorf("ToIcinga2Api called on unknown type %T", value))
	}
}
