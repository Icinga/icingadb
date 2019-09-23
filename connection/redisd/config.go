package redisd

import log "github.com/sirupsen/logrus"

// configLogLevels maps the logrus log levels to Redis log levels (as in config).
var configLogLevels = map[log.Level]string{
	log.FatalLevel: "warning",
	log.ErrorLevel: "warning",
	log.WarnLevel:  "warning",
	log.InfoLevel:  "notice",
	log.DebugLevel: "verbose",
}

// configTemplate is the constant part of Server's Redis config.
const configTemplate = `
daemonize no
supervised no

loglevel %s
logfile ""
syslog-enabled no

dir "%s"


unixsocket "%s"
unixsocketperm 700

port 0
bind 127.0.0.1
protected-mode yes

timeout 0


databases 1

save ""

stop-writes-on-bgsave-error yes

rdbcompression yes
rdbchecksum yes
dbfilename dump.rdb

appendonly no


maxclients 42

lua-time-limit 0

hash-max-ziplist-entries 512
hash-max-ziplist-value 64
list-max-ziplist-size -2
list-compress-depth 0
set-max-intset-entries 512
zset-max-ziplist-entries 128
zset-max-ziplist-value 64
hll-sparse-max-bytes 3000
activerehashing yes

client-output-buffer-limit normal 0 0 0
client-output-buffer-limit pubsub 0 0 0
`
