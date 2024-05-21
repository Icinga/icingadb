package redis

// Streams represents a Redis stream key to ID mapping.
type Streams map[string]string

// Option returns the Redis stream key to ID mapping
// as a slice of stream keys followed by their IDs
// that is compatible for the Redis STREAMS option.
func (s Streams) Option() []string {
	// len*2 because we're appending the IDs later.
	streams := make([]string, 0, len(s)*2)
	ids := make([]string, 0, len(s))

	for key, id := range s {
		streams = append(streams, key)
		ids = append(ids, id)
	}

	return append(streams, ids...)
}
