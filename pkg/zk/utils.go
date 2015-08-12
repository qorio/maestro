package zk

import (
	"encoding/json"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/registry"
	"strconv"
	"strings"
)

// Node value
func Follow(zc ZK, key registry.Path) (*Node, error) {
	n, err := zc.Get(key.Path())
	if err != nil {
		return nil, err
	}
	if strings.Index(n.GetValueString(), "env://") == 0 {
		next := n.GetValueString()[len("env://"):]
		return Follow(zc, registry.Path(next))
	} else {
		return n, nil
	}
}

// If value begins with env:// then automatically resolve the pointer recursively.
// Returns key, value, error
func Resolve(zc ZK, key registry.Path, value string) (registry.Path, string, error) {
	// de-reference the pointer...
	if strings.Index(value, "env://") == 0 {
		p := value[len("env://"):]
		n, err := zc.Get(p)
		switch {
		case err == ErrNotExist:
			return key, "", nil
		case err != nil:
			return key, "", err
		}
		glog.Infoln("Resolving", key, "=", value, "==>", n.GetValueString())
		return Resolve(zc, key, n.GetValueString())
	} else {
		return key, value, nil
	}
}

func PathExists(zc ZK, key registry.Path) bool {
	_, err := zc.Get(key.Path())
	switch {
	case err == ErrNotExist:
		return false
	case err != nil:
		return true
	}
	return true
}

func GetObject(zc ZK, key registry.Path, value interface{}) error {
	n, err := zc.Get(key.Path())
	switch {
	case err == ErrNotExist:
		return nil
	case err != nil:
		return nil
	}
	return json.Unmarshal(n.GetValue(), value)
}

func GetString(zc ZK, key registry.Path) *string {
	n, err := zc.Get(key.Path())
	switch {
	case err == ErrNotExist:
		return nil
	case err != nil:
		return nil
	}
	v := n.GetValueString()
	if v == "" {
		return nil
	}
	return &v
}

func GetBytes(zc ZK, key registry.Path) []byte {
	n, err := zc.Get(key.Path())
	switch {
	case err == ErrNotExist:
		return nil
	case err != nil:
		return nil
	}
	return n.GetValue()
}

func CreateOrSet(zc ZK, key registry.Path, value interface{}) error {
	switch value.(type) {
	case string:
		return CreateOrSetString(zc, key, value.(string))
	case []byte:
		return CreateOrSetBytes(zc, key, value.([]byte))
	default:
		serialized, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return CreateOrSetBytes(zc, key, serialized)
	}
}

func CreateOrSetString(zc ZK, key registry.Path, value string) error {
	return CreateOrSetBytes(zc, key, []byte(value))
}

func CreateOrSetBytes(zc ZK, key registry.Path, value []byte) error {
	n, err := zc.Get(key.Path())
	switch {
	case err == ErrNotExist:
		n, err = zc.Create(key.Path(), value)
		if err != nil {
			return err
		}
	case err != nil:
		return err
	}
	err = n.Set(value)
	if err != nil {
		return err
	}
	return nil
}

func Increment(zc ZK, key registry.Path, increment int) error {
	n, err := zc.Get(key.Path())
	switch {
	case err == ErrNotExist:
		n, err = zc.Create(key.Path(), []byte(strconv.Itoa(0)))
		if err != nil {
			return err
		}
	case err != nil:
		return err
	}
	count, err := strconv.Atoi(n.GetValueString())
	if err != nil {
		count = 0
	}
	err = n.Set([]byte(strconv.Itoa(count + 1)))
	if err != nil {
		return err
	}
	return nil
}

func DeleteObject(zc ZK, key registry.Path) error {
	err := zc.Delete(key.Path())
	switch err {
	case ErrNotExist:
		return nil
	default:
		return err
	}
}
