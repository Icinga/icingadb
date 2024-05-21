package database

// Entity is implemented by each type that works with the database package.
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
type ID interface {
	// String returns the string representation form of the ID.
	// The String method is used to use the ID in functions
	// where it needs to be compared or hashed.
	String() string
}

// IDer is implemented by every entity that uniquely identifies itself.
type IDer interface {
	ID() ID   // ID returns the ID.
	SetID(ID) // SetID sets the ID.
}

// EntityFactoryFunc knows how to create an Entity.
type EntityFactoryFunc func() Entity

// Upserter implements the Upsert method,
// which returns a part of the object for ON DUPLICATE KEY UPDATE.
type Upserter interface {
	Upsert() any // Upsert partitions the object.
}

// TableNamer implements the TableName method,
// which returns the table of the object.
type TableNamer interface {
	TableName() string // TableName tells the table.
}

// Scoper implements the Scope method,
// which returns a struct specifying the WHERE conditions that
// entities must satisfy in order to be SELECTed.
type Scoper interface {
	Scope() any
}

// PgsqlOnConflictConstrainter implements the PgsqlOnConflictConstraint method,
// which returns the primary or unique key constraint name of the PostgreSQL table.
type PgsqlOnConflictConstrainter interface {
	// PgsqlOnConflictConstraint returns the primary or unique key constraint name of the PostgreSQL table.
	PgsqlOnConflictConstraint() string
}
