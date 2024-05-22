package redis

import (
	"context"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
)

// WrapCmdErr adds the command itself and
// the stack of the current goroutine to the command's error if any.
func WrapCmdErr(cmd redis.Cmder) error {
	err := cmd.Err()
	if err != nil {
		err = errors.Wrapf(err, "can't perform %q", utils.Ellipsize(
			redis.NewCmd(context.Background(), cmd.Args()).String(), // Omits error in opposite to cmd.String()
			100,
		))
	}

	return err
}
