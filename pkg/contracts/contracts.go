package contracts

import "crypto/sha1"

// Entity is implemented by every type Icinga DB should synchronize.
type Entity interface {
	Fingerprinter
	IDer
}

// Fingerprinter is implemented by every entity that uniquely identifies itself.
type Fingerprinter interface {
	// Fingerprint returns the value that uniquely identifies the entity.
	Fingerprint() Fingerprinter
}

// ID is a unique identifier of an entity.
type ID [sha1.Size]byte

// IDer is implemented by every entity that uniquely identifies itself.
type IDer interface {
	ID() ID   // ID returns the ID.
	SetID(ID) // SetID sets the ID.
}

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

// EntityFactoryFunc knows how to create an Entity.
type EntityFactoryFunc func() Entity

// WithInit calls Init() on the created Entity if applicable.
func (f EntityFactoryFunc) WithInit() Entity {
	e := f()

	if initer, ok := e.(Initer); ok {
		initer.Init()
	}

	return e
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

// Upserter implements the Upsert method,
// which returns a part of the object for ON DUPLICATE KEY UPDATE.
type Upserter interface {
	Upsert() interface{} // Upsert partitions the object.
}

// TableNamer implements the TableName method,
// which returns the table of the object.
type TableNamer interface {
	TableName() string // TableName tells the table.
}
