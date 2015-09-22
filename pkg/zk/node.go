package zk

import (
	"github.com/samuel/go-zookeeper/zk"
	"path/filepath"
	"strconv"
)

type Node struct {
	Path    string
	Value   []byte
	Members []string
	Stats   *zk.Stat
	Leaf    bool
	zk      *zookeeper
}

func (z *Node) GetPath() string {
	return z.Path
}

func (z *Node) GetBasename() string {
	return filepath.Base(z.Path)
}

func (z *Node) GetValue() []byte {
	return z.Value
}

func (z *Node) GetValueString() string {
	return string(z.Value)
}

func (z *Node) IsLeaf() bool {
	return z.Leaf
}

func (this *Node) Get() error {
	if err := this.zk.check(); err != nil {
		return err
	}
	v, s, err := this.zk.conn.Get(this.Path)
	if err != nil {
		return filter_err(err)
	}
	this.Value = v
	this.Stats = s
	return nil
}

func (this *Node) Watch(f func(Event)) (chan<- bool, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	value, stat, event_chan, err := this.zk.conn.GetW(this.Path)
	if err != nil {
		return nil, filter_err(err)
	}
	this.Value = value
	this.Stats = stat
	return run_watch(f, event_chan)
}

func (this *Node) WatchChildren(f func(Event)) (chan<- bool, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	members, stat, event_chan, err := this.zk.conn.ChildrenW(this.Path)
	if err != nil {
		return nil, filter_err(err)
	}
	this.Members = members
	this.Stats = stat
	return run_watch(f, event_chan)
}

func (this *Node) Set(value []byte) error {
	if err := this.zk.check(); err != nil {
		return err
	}
	s, err := this.zk.conn.Set(this.Path, value, this.Stats.Version)
	if err != nil {
		return filter_err(err)
	}
	this.Value = value
	this.Stats = s
	this.zk.track_ephemeral(this, s.EphemeralOwner > 0)
	return nil
}

func (this *Node) CountChildren() int32 {
	if this.Stats == nil {
		if err := this.Get(); err != nil {
			return -1
		}
	}
	return this.Stats.NumChildren
}

func (this *Node) Children() ([]*Node, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	paths, s, err := this.zk.conn.Children(this.Path)
	if err != nil {
		return nil, err
	} else {
		this.Stats = s
		children := make([]*Node, len(paths))
		for i, p := range paths {
			children[i] = &Node{Path: this.Path + "/" + p, zk: this.zk}
			err := children[i].Get()
			if err != nil {
				return nil, err
			}
		}
		return children, nil
	}
}

func (this *Node) ListAllRecursive() ([]string, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]string, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}
	for _, n := range children {
		l, err := n.ListAllRecursive()
		if err != nil {
			return nil, err
		}
		list = append_string_slices(list, l)
		list = append(list, n.Path)
	}
	return list, nil
}

func (this *Node) ChildrenRecursive() ([]*Node, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]*Node, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}

	this.Leaf = len(children) == 0

	for _, n := range children {
		l, err := n.ChildrenRecursive()
		if err != nil {
			return nil, err
		}
		list = append_node_slices(list, l)
		list = append(list, n)
	}
	return list, nil
}

// Recursively go through all the children.  Apply filter for each node. If filter returns
// true for the particular node, this node (though not necessarily all its children) will be
// excluded.  This is useful for searching through all true by name or by whether it's a parent
// node or not.
func (this *Node) FilterChildrenRecursive(filter func(*Node) bool) ([]*Node, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]*Node, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}

	this.Leaf = len(children) == 0

	for _, n := range children {
		l, err := n.FilterChildrenRecursive(filter)
		if err != nil {
			return nil, err
		}
		list = append_node_slices(list, l)
		add := filter == nil || (filter != nil && !filter(n))
		if add {
			list = append(list, n)
		}
	}
	return list, nil
}

func (this *Node) VisitChildrenRecursive(accept func(*Node) bool) ([]*Node, error) {
	if err := this.zk.check(); err != nil {
		return nil, err
	}
	list := make([]*Node, 0)

	children, err := this.Children()
	if err != nil {
		return nil, err
	}

	this.Leaf = len(children) == 0
	for _, n := range children {
		l, err := n.VisitChildrenRecursive(accept)
		if err != nil {
			return nil, err
		}
		list = append_node_slices(list, l)
		if accept == nil || (accept != nil && accept(n)) {
			list = append(list, n)
		}
	}
	return list, nil
}

func (this *Node) Delete() error {
	if err := this.zk.check(); err != nil {
		return err
	}
	err := this.zk.Delete(this.Path)
	if err != nil {
		return err
	} else {
		return nil
	}
}

func (this *Node) Increment(increment int) (int, error) {
	if err := this.zk.check(); err != nil {
		return -1, err
	}
	count, err := strconv.Atoi(this.GetValueString())
	if err != nil {
		count = 0
	}
	count += increment
	err = this.Set([]byte(strconv.Itoa(count)))
	if err != nil {
		return -1, err
	}
	return count, nil
}

func (this *Node) CheckAndIncrement(current, increment int) (int, error) {
	if err := this.zk.check(); err != nil {
		return -1, err
	}
	count, err := strconv.Atoi(this.GetValueString())
	switch {
	case err != nil:
		return -1, err
	case count != current:
		return -1, ErrConflict
	}
	count += increment
	err = this.Set([]byte(strconv.Itoa(count)))
	if err != nil {
		return -1, err
	}
	return count, nil
}
