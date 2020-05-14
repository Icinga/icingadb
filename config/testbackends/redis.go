package testbackends

import (
	"fmt"
	"github.com/go-redis/redis/v7"
	"os"
	"time"
)

var RedisTestAddr = fmt.Sprintf("%s:%s", os.Getenv("ICINGADB_TEST_REDIS_HOST"), os.Getenv("ICINGADB_TEST_REDIS_PORT"))

var RedisTestClient = redis.NewClient(&redis.Options{
	Network:      "tcp",
	Addr:         RedisTestAddr,
	ReadTimeout:  time.Minute,
	WriteTimeout: time.Minute,
})
