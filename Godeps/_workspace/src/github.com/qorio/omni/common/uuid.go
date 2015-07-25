package common

import (
	"code.google.com/p/go-uuid/uuid"
	"database/sql/driver"
)

type UUID uuid.UUID

func NewUUID() UUID {
	return UUID(uuid.NewUUID())
}

func UUIDFromString(s string) UUID {
	return UUID(uuid.Parse(s))
}

func (i UUID) String() string {
	return uuid.UUID(i).String()
}

func Equals(uuid1, uuid2 UUID) bool {
	return uuid.Equal(uuid.UUID(uuid1), uuid.UUID(uuid2))
}

// For custom mapping to sql data base
func (id UUID) Value() (driver.Value, error) {
	//Assuming that p.String() returns a correctly formatted string
	return id.String(), nil
}

func (id UUID) Scan(val interface{}) error {
	// parse []byte
	if buff, ok := val.([]byte); ok {
		copy(id, buff)
	}
	return nil
}
