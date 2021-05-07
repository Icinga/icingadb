package value

import "fmt"

type NotificationTypes []string

func (t NotificationTypes) IcingaDbValue() interface{} {
	v := uint(0)

	for _, s := range t {
		if bit, ok := notificationTypeMap[s]; ok {
			v |= bit
		} else {
			panic(fmt.Errorf("unknown notification type %q", s))
		}
	}

	return v
}

func (t NotificationTypes) Icinga2ConfigValue() string {
	return ToIcinga2Config([]string(t))
}

func (t NotificationTypes) Icinga2ApiValue() interface{} {
	return ToIcinga2Api([]string(t))
}

// https://github.com/Icinga/icinga2/blob/a8f98cf72115d50152137bc924277b426f483a3f/lib/icinga/notification.hpp#L34-L50
var notificationTypeMap = map[string]uint{
	"DowntimeStart":   1,
	"DowntimeEnd":     2,
	"DowntimeRemoved": 4,
	"Custom":          8,
	"Acknowledgement": 16,
	"Problem":         32,
	"Recovery":        64,
	"FlappingStart":   128,
	"FlappingEnd":     256,
}
