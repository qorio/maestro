package zk

import (
	"encoding/json"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/registry"
	"strconv"
	"strings"
)

const (
	PrefixEnv = "env://"
	PrefixZk  = "zk://"
)

// Node value
func Follow(zc ZK, key registry.Path) (*Node, error) {
	n, err := zc.Get(key.Path())
	if err != nil {
		return nil, err
	}

	switch {
	case strings.Index(n.GetValueString(), PrefixEnv) == 0:
		next := n.GetValueString()[len(PrefixEnv):]
		return Follow(zc, registry.Path(next))
	case strings.Index(n.GetValueString(), PrefixZk) == 0:
		next := n.GetValueString()[len(PrefixZk):]
		return Follow(zc, registry.Path(next))
	default:
		return n, nil
	}
}

// If value begins with env:// then automatically resolve the pointer recursively.
// Returns key, value, error
func Resolve(zc ZK, key registry.Path, value string) (registry.Path, string, error) {
	// de-reference the pointer...
	switch {
	case strings.Index(value, PrefixEnv) == 0:
		p := value[len(PrefixEnv):]
		n, err := zc.Get(p)
		switch {
		case err == ErrNotExist:
			return key, "", nil
		case err != nil:
			return key, "", err
		}
		glog.Infoln("Resolving", key, "=", value, "==>", n.GetValueString())
		return Resolve(zc, key, n.GetValueString())
	case strings.Index(value, PrefixZk) == 0:
		p := value[len(PrefixZk):]
		n, err := zc.Get(p)
		switch {
		case err == ErrNotExist:
			return key, "", nil
		case err != nil:
			return key, "", err
		}
		glog.Infoln("Resolving", key, "=", value, "==>", n.GetValueString())
		return Resolve(zc, key, n.GetValueString())
	default:
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

func GetInt(zc ZK, key registry.Path) *int {
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
	i, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &i
}

func CreateOrSet(zc ZK, key registry.Path, value interface{}, ephemeral ...bool) error {
	switch value.(type) {
	case string:
		return CreateOrSetString(zc, key, value.(string), ephemeral...)
	case []byte:
		return CreateOrSetBytes(zc, key, value.([]byte), ephemeral...)
	default:
		serialized, err := json.Marshal(value)
		if err != nil {
			return err
		}
		return CreateOrSetBytes(zc, key, serialized, ephemeral...)
	}
}

func CreateOrSetInt(zc ZK, key registry.Path, value int, ephemeral ...bool) error {
	v := strconv.Itoa(value)
	return CreateOrSetBytes(zc, key, []byte(v), ephemeral...)
}

func CreateOrSetString(zc ZK, key registry.Path, value string, ephemeral ...bool) error {
	return CreateOrSetBytes(zc, key, []byte(value), ephemeral...)
}

func CreateOrSetBytes(zc ZK, key registry.Path, value []byte, ephemeral ...bool) error {
	if len(ephemeral) > 0 && ephemeral[0] {
		_, err := zc.CreateEphemeral(key.Path(), value)
		return err
	}

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
	_, err = n.Increment(increment)
	return err
}

func CheckAndIncrement(zc ZK, key registry.Path, current, increment int) (int, error) {
	n, err := zc.Get(key.Path())
	switch {
	case err == ErrNotExist, len(n.GetValue()) == 0:
		val := 0
		n, err = zc.Create(key.Path(), []byte(strconv.Itoa(val)))
		if err != nil {
			return -1, err
		}
		return val, nil
	case err != nil:
		return -1, err
	}
	return n.CheckAndIncrement(current, increment)
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

func Visit(zc ZK, key registry.Path, v func(registry.Path, []byte) bool) error {
	zn, err := zc.Get(key.Path())
	if err != nil {
		return err
	}
	children, err := zn.Children()
	if err != nil {
		return err
	}
	for _, n := range children {
		if !v(registry.NewPath(n.GetPath()), n.GetValue()) {
			return nil
		}
	}
	return nil
}

// A simple non-ephemeral lock held at key and we use simply by incrementing and
// using it like a compare and swap.
func VersionLockAndExecute(zc ZK, key registry.Path, rev int, f func() error) (int, error) {
	cas, err := CheckAndIncrement(zc, key, rev, 1)
	if err != nil {
		return -1, ErrConflict
	}
	if err := f(); err != nil {
		return -1, err
	}
	return CheckAndIncrement(zc, key, cas, 1)
}
