package flatten

import (
	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFlatten(t *testing.T) {
	for _, st := range []struct {
		name   string
		prefix string
		value  any
		output map[string]types.String
	}{
		{"nil", "a", nil, map[string]types.String{"a": types.MakeString("null")}},
		{"bool", "b", true, map[string]types.String{"b": types.MakeString("true")}},
		{"int", "c", 42, map[string]types.String{"c": types.MakeString("42")}},
		{"float", "d", 77.7, map[string]types.String{"d": types.MakeString("77.7")}},
		{"large_float", "e", 1e23, map[string]types.String{"e": types.MakeString("100000000000000000000000")}},
		{"string", "f", "\x00", map[string]types.String{"f": types.MakeString("\x00")}},
		{"nil_slice", "g", []any(nil), map[string]types.String{"g": {}}},
		{"empty_slice", "h", []any{}, map[string]types.String{"h": {}}},
		{"slice", "i", []any{nil}, map[string]types.String{"i[0]": types.MakeString("null")}},
		{"nil_map", "j", map[string]any(nil), map[string]types.String{"j": {}}},
		{"empty_map", "k", map[string]any{}, map[string]types.String{"k": {}}},
		{"map", "l", map[string]any{" ": nil}, map[string]types.String{"l. ": types.MakeString("null")}},
		{"map_with_slice", "m", map[string]any{"\t": []any{"ä", "ö", "ü"}, "ß": "s"}, map[string]types.String{
			"m.\t[0]": types.MakeString("ä"),
			"m.\t[1]": types.MakeString("ö"),
			"m.\t[2]": types.MakeString("ü"),
			"m.ß":     types.MakeString("s"),
		}},
		{"slice_with_map", "n", []any{map[string]any{"ä": "a", "ö": "o", "ü": "u"}, "ß"}, map[string]types.String{
			"n[0].ä": types.MakeString("a"),
			"n[0].ö": types.MakeString("o"),
			"n[0].ü": types.MakeString("u"),
			"n[1]":   types.MakeString("ß"),
		}},
	} {
		t.Run(st.name, func(t *testing.T) {
			assert.Equal(t, st.output, Flatten(st.value, st.prefix))
		})
	}
}
