package common

import (
	"github.com/bmizerany/assert"
	"testing"
)

func TestUUID(t *testing.T) {
	uuid1 := NewUUID()
	uuid2 := UUIDFromString(uuid1.String())

	assert.Equal(t, uuid1, uuid2)
	assert.Equal(t, uuid1.String(), uuid2.String())
}
