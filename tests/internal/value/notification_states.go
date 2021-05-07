package value

import "fmt"

type NotificationStates []string

func (s NotificationStates) IcingaDbValue() interface{} {
	v := uint(0)

	for _, s := range s {
		if bit, ok := notificationStateMap[s]; ok {
			v |= bit
		} else {
			panic(fmt.Errorf("unknown notification state %q", s))
		}
	}

	return v
}

func (s NotificationStates) Icinga2ConfigValue() string {
	return ToIcinga2Config([]string(s))
}

func (s NotificationStates) Icinga2ApiValue() interface{} {
	return ToIcinga2Api([]string(s))
}

// https://github.com/Icinga/icinga2/blob/a8f98cf72115d50152137bc924277b426f483a3f/lib/icinga/notification.hpp#L20-L32
var notificationStateMap = map[string]uint{
	"OK":       1,
	"Warning":  2,
	"Critical": 4,
	"Unknown":  8,
	"Up":       16,
	"Down":     32,
}
