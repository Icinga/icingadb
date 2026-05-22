package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/icinga/icinga-go-library/types"
	"github.com/stretchr/testify/assert"
	"github.com/theory/jsonpath/spec"
)

func Test_relations_complete(t *testing.T) {
	t.Run("demo provider", func(t *testing.T) {
		rel := &relations{
			hostId: types.Binary([]byte{0xc0, 0xff, 0xee}),

			provider: func(_ context.Context, rel *relations, segments []*spec.Segment) error {
				if !strings.Contains(fmt.Sprintf("%+v", segments), "vars") {
					// Initial call: construct host object.
					rel.Host = &icingaObject{
						Name:        "example.com",
						DisplayName: "example.com",
					}
					rel.completeRelations = append(rel.completeRelations, "host.name", "host.display_name")
				} else {
					// Subsequent call: populate custom vars.
					rel.Host.Vars = map[string]any{
						"env": "prod",
						"os":  "Hannah Montana Linux",
					}
					rel.completeRelations = append(rel.completeRelations, "host.vars")
				}
				return nil
			},

			Object: struct{ Type string }{"host"},
		}

		assert.NoError(t, rel.complete(t.Context(), "$.host.name"))
		assert.Equal(t,
			&icingaObject{
				Name:        "example.com",
				DisplayName: "example.com",
			},
			rel.Host)
		assert.Equal(t, []string{"host.name", "host.display_name"}, rel.completeRelations)

		assert.NoError(t, rel.complete(t.Context(), "$.host.vars"))
		assert.Equal(t,
			&icingaObject{
				Name:        "example.com",
				DisplayName: "example.com",
				Vars:        map[string]any{"env": "prod", "os": "Hannah Montana Linux"},
			},
			rel.Host)
		assert.Equal(t, []string{"host.name", "host.display_name", "host.vars"}, rel.completeRelations)

		jsonOut, err := json.Marshal(rel.asMap())
		assert.NoError(t, err)
		assert.JSONEq(t,
			`{
				"object": {"type": "host"},
				"host": {
					"name": "example.com",
					"display_name": "example.com",
					"vars": {
						"env": "prod",
						"os": "Hannah Montana Linux"
					}
				}
			}`,
			string(jsonOut))
	})

	t.Run("jsonpath invalid", func(t *testing.T) {
		rel := &relations{
			provider: func(_ context.Context, _ *relations, _ []*spec.Segment) error {
				t.Fatal("provider should not have been called")
				return nil
			},
		}

		err := rel.complete(t.Context(), "$[invalid")
		assert.Error(t, err)
	})
}
