package notifications

import (
	"crypto/sha1" // #nosec G505 -- Blocklisted import crypto/sha1
	"fmt"

	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/objectpacker"
	"github.com/icinga/icinga-go-library/types"
	v1 "github.com/icinga/icingadb/pkg/icingadb/v1"
)

// StateToSeverity converts a state integer to an event.Severity value.
func StateToSeverity(s *v1.State, isService bool) (event.Severity, error) {
	if isService {
		switch s.HardState {
		case 0:
			return event.SeverityOK, nil
		case 1:
			return event.SeverityWarning, nil
		case 2:
			return event.SeverityCrit, nil
		case 3:
			return event.SeverityErr, nil
		default:
			return event.SeverityNone, fmt.Errorf("unexpected service state %d", s.HardState)
		}
	} else {
		switch s.HardState {
		case 0:
			return event.SeverityOK, nil
		case 1:
			return event.SeverityCrit, nil
		default:
			return event.SeverityNone, fmt.Errorf("unexpected host state %d", s.HardState)
		}
	}
}

// HaveSameState checks if the given incident and the corresponding [database.Entity] have the same state.
//
// This function is used to determine if an incident in Icinga Notifications corresponds to the current state
// of a checkable in Icinga DB. It compares the severity and muted status of the incident with the state of the
// entity and returns true if they match, false otherwise. If the entity type is unsupported, an error is returned.
func HaveSameState(incident source.Incident, entity database.Entity) (bool, error) {
	var s *v1.State
	var isService bool
	switch e := entity.(type) {
	case *v1.HostState:
		s = &e.State
	case *v1.ServiceState:
		s = &e.State
		isService = true
	default:
		return false, fmt.Errorf("unsupported entity type %T", entity)
	}

	severity, err := StateToSeverity(s, isService)
	if err != nil {
		return false, err
	}

	if incident.Severity != severity {
		return false, nil
	}

	inDowntime := s.InDowntime.Valid && s.InDowntime.Bool
	isAcked := s.IsAcknowledged.Valid && s.IsAcknowledged.Bool
	isFlapping := s.IsFlapping.Valid && s.IsFlapping.Bool
	if incident.IsMuted != (inDowntime || isAcked || isFlapping) {
		return false, nil
	}
	return true, nil
}

// DeriveDeterministicEventUUID generates a UUID based on the given environment ID, object name, event type, and timestamp.
//
// The generated UUID is deterministic and can be used to uniquely identify events in Icinga Notifications.
// It also returns the SHA1 hash of the input data, which can be used for other purposes if needed.
func DeriveDeterministicEventUUID(envId types.Binary, eventType, objectName string, ts types.UnixMilli) (types.UUID, types.Binary, error) {
	hash := sha1.New() // #nosec G401 -- used as a non-cryptographic hash function to hash IDs.
	data := []any{envId.String(), eventType, objectName}
	if !ts.Time().IsZero() {
		data = append(data, float64(ts.Time().UnixMilli()))
	}
	if err := objectpacker.PackAny(data, hash); err != nil {
		return types.UUID{}, nil, err
	}
	// This will rehash the already hashed data to generate the final UUID, which is kind of redundant,
	// but it must be done this way as SyncAlertHistories would otherwise have no way to reproduce the
	// same UUIDs from history.id values, which are already SHA1 hashes of the same data.
	return types.MakeUUID(uuid.NewSHA1(uuid.Nil, hash.Sum(nil))), hash.Sum(nil), nil
}

// GenSHA1 generates a SHA1 hash based on the given environment ID, host name, and service name.
//
// This implementation mimics the Icinga 2 ID generation behavior[^1] used to generate all Icinga DB
// related object IDs, so make sure to keep it in sync with the Icinga 2 implementation.
//
// [^1]: https://github.com/Icinga/icinga2/blob/v2.16.3/lib/icingadb/icingadb-utility.cpp#L81
func GenSHA1(envId types.Binary, host, service string) (types.Binary, error) {
	objectName := host
	if service != "" {
		objectName += "!" + service
	}

	hash := sha1.New() // #nosec G401 -- used as a non-cryptographic hash function to hash IDs.
	if err := objectpacker.PackAny([]string{envId.String(), objectName}, hash); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

// Ternary returns the first argument if the condition is true, otherwise it returns the second argument.
func Ternary[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}
