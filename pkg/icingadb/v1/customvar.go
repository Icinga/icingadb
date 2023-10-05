package v1

import (
	"context"
	"github.com/icinga/icingadb/internal"
	"github.com/icinga/icingadb/pkg/com"
	"github.com/icinga/icingadb/pkg/contracts"
	"github.com/icinga/icingadb/pkg/flatten"
	"github.com/icinga/icingadb/pkg/objectpacker"
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
	Flatvalue        types.String `json:"flatvalue"`
}

func NewCustomvar() contracts.Entity {
	return &Customvar{}
}

func NewCustomvarFlat() contracts.Entity {
	return &CustomvarFlat{}
}

// ExpandCustomvars streams custom variables from a provided channel and returns three channels,
// the first providing the unmodified custom variable read from the input channel,
// the second channel providing the corresponding resolved flat custom variables,
// and the third channel providing an error, if any.
func ExpandCustomvars(
	ctx context.Context,
	cvs <-chan contracts.Entity,
) (customvars, flatCustomvars <-chan contracts.Entity, errs <-chan error) {
	g, ctx := errgroup.WithContext(ctx)

	// Multiplex cvs to use them both for customvar and customvar_flat.
	var forward chan contracts.Entity
	customvars, forward = multiplexCvs(ctx, g, cvs)
	flatCustomvars = flattenCustomvars(ctx, g, forward)
	errs = com.WaitAsync(g)

	return
}

// multiplexCvs streams custom variables from a provided channel and
// forwards each custom variable to the two returned output channels.
func multiplexCvs(
	ctx context.Context,
	g *errgroup.Group,
	cvs <-chan contracts.Entity,
) (customvars1, customvars2 chan contracts.Entity) {
	customvars1 = make(chan contracts.Entity)
	customvars2 = make(chan contracts.Entity)

	g.Go(func() error {
		defer close(customvars1)
		defer close(customvars2)

		for {
			select {
			case cv, ok := <-cvs:
				if !ok {
					return nil
				}

				select {
				case customvars1 <- cv:
				case <-ctx.Done():
					return ctx.Err()
				}

				select {
				case customvars2 <- cv:
				case <-ctx.Done():
					return ctx.Err()
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	return
}

// flattenCustomvars creates and yields flat custom variables from the provided custom variables.
func flattenCustomvars(ctx context.Context, g *errgroup.Group, cvs <-chan contracts.Entity) (flatCustomvars chan contracts.Entity) {
	flatCustomvars = make(chan contracts.Entity)

	g.Go(func() error {
		defer close(flatCustomvars)

		g, ctx := errgroup.WithContext(ctx)

		for i := 0; i < runtime.NumCPU(); i++ {
			g.Go(func() error {
				for entity := range cvs {
					var value interface{}
					customvar := entity.(*Customvar)
					if err := internal.UnmarshalJSON([]byte(customvar.Value), &value); err != nil {
						return err
					}

					flattened := flatten.Flatten(value, customvar.Name)

					for flatname, flatvalue := range flattened {
						var fv interface{}
						if flatvalue.Valid {
							fv = flatvalue.String
						}

						select {
						case flatCustomvars <- &CustomvarFlat{
							CustomvarMeta: CustomvarMeta{
								EntityWithoutChecksum: EntityWithoutChecksum{
									IdMeta: IdMeta{
										// TODO(el): Schema comment is wrong.
										// Without customvar.Id we would produce duplicate keys here.
										Id: utils.Checksum(objectpacker.MustPackSlice(customvar.EnvironmentId, customvar.Id, flatname, fv)),
									},
								},
								EnvironmentMeta: EnvironmentMeta{
									EnvironmentId: customvar.EnvironmentId,
								},
								CustomvarId: customvar.Id,
							},
							Flatname:         flatname,
							FlatnameChecksum: utils.Checksum(flatname),
							Flatvalue:        flatvalue,
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

	return
}
