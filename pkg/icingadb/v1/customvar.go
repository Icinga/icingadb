package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/flatten"
	"github.com/icinga/icingadb/pkg/icingadb/objectpacker"
	"github.com/icinga/icingadb/pkg/types"
	"github.com/icinga/icingadb/pkg/utils"
	"golang.org/x/sync/errgroup"
	"runtime"
)

type Customvar struct {
	EntityWithoutChecksum `json:",inline"`
	EnvironmentMeta       `json:",inline"`
	NameMeta              `json:",inline"`
	Value                 string `json:"value"`
}

type CustomvarFlat struct {
	CustomvarMeta    `json:",inline"`
	Flatname         string       `json:"flatname"`
	FlatnameChecksum types.Binary `json:"flatname_checksum"`
	Flatvalue        string       `json:"flatvalue"`
}

func NewCustomvar() contracts.Entity {
	return &Customvar{}
}

func NewCustomvarFlat() contracts.Entity {
	return &CustomvarFlat{}
}

// FlattenCustomvars creates and yields flat custom variables from the provided custom variables.
func FlattenCustomvars(ctx context.Context, cvs <-chan contracts.Entity) (<-chan contracts.Entity, <-chan error) {
	cvFlats := make(chan contracts.Entity)
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer close(cvFlats)

		g, _ := errgroup.WithContext(ctx)

		for i := 0; i < runtime.NumCPU(); i++ {
			g.Go(func() error {
				for entity := range cvs {
					var value interface{}
					customvar := entity.(*Customvar)
					if err := json.Unmarshal([]byte(customvar.Value), &value); err != nil {
						return err
					}

					flattened := flatten.Flatten(value, customvar.Name)

					for flatname, flatvalue := range flattened {
						var fv string
						if flatvalue == nil {
							fv = "null"
						} else {
							fv = fmt.Sprintf("%v", flatvalue)
						}

						select {
						case cvFlats <- &CustomvarFlat{
							CustomvarMeta: CustomvarMeta{
								EntityWithoutChecksum: EntityWithoutChecksum{
									IdMeta: IdMeta{
										// TODO(el): Schema comment is wrong.
										// Without customvar.Id we would produce duplicate keys here.
										Id: utils.Checksum(objectpacker.MustPackAny(customvar.EnvironmentId, customvar.Id, flatname, flatvalue)),
									},
								},
								EnvironmentMeta: EnvironmentMeta{
									EnvironmentId: customvar.EnvironmentId,
								},
								CustomvarId: customvar.Id,
							},
							Flatname:         flatname,
							FlatnameChecksum: utils.Checksum(flatname),
							Flatvalue:        fv,
						}:
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}

				return nil
			})
		}

		return g.Wait()
	})

	return cvFlats, com.WaitAsync(g)
}
