-- get_overdues.lua takes the following KEYS:
-- * either icinga:nextupdate:host or icinga:nextupdate:service
-- * either icingadb:overdue:host or icingadb:overdue:service
-- * a random one
--
-- It takes the following ARGV:
-- * the current date and time as *nix timestamp float in seconds
--
-- It returns the following:
-- * overdue monitored objects not yet marked overdue
-- * not overdue monitored objects not yet unmarked overdue

local icingaNextupdate = KEYS[1]
local icingadbOverdue = KEYS[2]
local tempOverdue = KEYS[3]
local now = ARGV[1]

redis.call('DEL', tempOverdue)

local zrbs = redis.call('ZRANGEBYSCORE', icingaNextupdate, '-inf', '(' .. now)
for i = 1, #zrbs do
    redis.call('SADD', tempOverdue, zrbs[i])
end
zrbs = nil

local res = { redis.call('SDIFF', tempOverdue, icingadbOverdue), redis.call('SDIFF', icingadbOverdue, tempOverdue) }

redis.call('DEL', tempOverdue)

return res
