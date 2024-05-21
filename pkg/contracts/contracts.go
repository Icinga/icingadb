package contracts

// Checksum is a unique identifier of an entity.
type Checksum interface {
	// String returns the string representation form of the Checksum.
	// The String method is used to use the Checksum in functions
	// where it needs to be compared or hashed.
	String() string
}

// Checksumer is implemented by every entity with a checksum.
type Checksumer interface {
	Checksum() Checksum   // Checksum returns the Checksum.
	SetChecksum(Checksum) // SetChecksum sets the Checksum.
}

// Initer implements the Init method,
// which initializes the object in addition to zeroing.
type Initer interface {
	Init() // Init initializes the object.
}

// SafeInit attempts to initialize the passed argument by calling its Init method,
// but only if the argument implements the [Initer] interface.
func SafeInit(v any) {
	if initer, ok := v.(Initer); ok {
		initer.Init()
	}
}
