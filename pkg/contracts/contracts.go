package contracts

// Equaler is implemented by every type that is comparable.
type Equaler interface {
	Equal(Equaler) bool // Equal checks for equality.
}

// Checksum is a unique identifier of an entity.
type Checksum interface {
	Equaler
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

// Waiter implements the Wait method,
// which blocks until execution is complete.
type Waiter interface {
	Wait() error // Wait waits for execution to complete.
}

// The WaiterFunc type is an adapter to allow the use of ordinary functions as Waiter.
// If f is a function with the appropriate signature, WaiterFunc(f) is a Waiter that calls f.
type WaiterFunc func() error

// Wait implements the Waiter interface.
func (f WaiterFunc) Wait() error {
	return f()
}

// Initer implements the Init method,
// which initializes the object in addition to zeroing.
type Initer interface {
	Init() // Init initializes the object.
}
