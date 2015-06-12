package zk

import (
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/registry"
	"strconv"
	"strings"
)

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

func GetValue(zc ZK, key registry.Path) *string {
	n, err := zc.Get(key.String())
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

func CreateOrSet(zc ZK, key registry.Path, value string) error {
	n, err := zc.Get(key.String())
	switch {
	case err == ErrNotExist:
		n, err = zc.Create(key.String(), []byte(value))
		if err != nil {
			return err
		}
	case err != nil:
		return err
	}
	err = n.Set([]byte(value))
	if err != nil {
		return err
	}
	return nil
}

func Increment(zc ZK, key registry.Path, increment int) error {
	n, err := zc.Get(key.String())
	switch {
	case err == ErrNotExist:
		n, err = zc.Create(key.String(), []byte(strconv.Itoa(0)))
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
