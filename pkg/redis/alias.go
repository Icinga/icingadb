package redis

import "github.com/redis/go-redis/v9"

// Alias definitions of commonly used go-redis exports,
// so that only this redis package needs to be imported and not go-redis additionally.

type IntCmd = redis.IntCmd
type Pipeliner = redis.Pipeliner
type XAddArgs = redis.XAddArgs
type XMessage = redis.XMessage
type XReadArgs = redis.XReadArgs

var NewScript = redis.NewScript
