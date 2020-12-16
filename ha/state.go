package ha

type State uint8

const (
	StateInit           State = iota // Initial state when starting
	StateActive                      // This instance is active
	StateOtherActive                 // This instance is inactive but there is another one that is active
	StateAllInactive                 // All known instances are inactive (i.e. none receives Icinga 2 heartbeats)
	StateInactiveUnkown              // This instance is inactive but does not known about the state of other instances
)

func (s State) String() string {
	switch s {
	case StateInit:
		return "init"
	case StateActive:
		return "active"
	case StateOtherActive:
		return "inactive (other instance active)"
	case StateAllInactive:
		return "inactive (all instances inactive)"
	case StateInactiveUnkown:
		return "inactive (other instances unkown)"
	default:
		return "(invalid)"
	}
}
