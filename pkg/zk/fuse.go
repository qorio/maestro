package zk

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
	"github.com/golang/glog"
	"github.com/qorio/maestro/pkg/registry"
	"golang.org/x/net/context"
	"os"
	"sync"
	"syscall"
)

type fsNode struct {
	Path registry.Path
	conn ZK
	node *Node
	Dir  *Dir
}

type FS struct {
	*fsNode
	fuse_conn  *fuse.Conn
	mountpoint *string
}

func NewFS(zc ZK, path registry.Path) *FS {
	return &FS{
		fsNode: fs_node(zc, nil, path, nil),
	}
}

func (this *FS) Unmount() error {
	if this.mountpoint != nil {
		err := fuse.Unmount(*this.mountpoint)
		glog.V(100).Infoln("Unmount dir=", this.mountpoint, "Err=", err)
		return err
	}
	return nil
}

func (this *FS) Shutdown() error {
	if err := this.Unmount(); err == nil {
		return this.fuse_conn.Close()
	} else {
		return err
	}
}

func (this *FS) Mount(dir string, perm os.FileMode) error {
	if err := os.MkdirAll(dir, perm); err != nil {
		return err
	}
	fc, err := fuse.Mount(dir)
	glog.V(100).Infoln("Mounting directory", dir, "Err=", err)
	if err != nil {
		return err
	}
	this.fuse_conn = fc
	this.mountpoint = &dir
	return fs.Serve(this.fuse_conn, this)
}

func (this *FS) Wait() error {
	if this.fuse_conn != nil {
		<-this.fuse_conn.Ready
	}
	return this.fuse_conn.MountError
}

type Dir struct {
	*fsNode
}

type File struct {
	*fsNode

	// lock
	mu sync.Mutex
	// number of write-capable handles currently open
	writers uint
}

var _ = fs.FS(&FS{})

func fs_node(zc ZK, n *Node, p registry.Path, d *Dir) *fsNode {
	return &fsNode{
		Dir:  d,
		Path: p,
		conn: zc,
		node: n,
	}
}

func (this *FS) Root() (fs.Node, error) {
	if this.node == nil {
		if n, err := Follow(this.conn, this.Path); err != nil {
			return nil, err
		} else {
			this.node = n
		}
	}
	n := &Dir{
		fsNode: fs_node(this.conn, this.node, this.Path, nil),
	}
	glog.V(100).Infoln("Root=", this.Path, "Node=", this.node, "Dir=", n)
	return n, nil
}

var _ = fs.Node(&Dir{})

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0755
	return nil
}

var _ = fs.HandleReadDirAller(&Dir{})

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var result []fuse.Dirent

	if children, err := d.node.Children(); err != nil {
		glog.Warningln("Error reading children of", d.node)
		return nil, err
	} else {
		for _, c := range children {
			de := fuse.Dirent{
				Name: c.GetBasename(),
			}
			if c.CountChildren() == 0 {
				de.Type = fuse.DT_File
			} else {
				de.Type = fuse.DT_Dir
			}
			result = append(result, de)
			glog.V(100).Infoln("Dirent=", de, "node=", c)
		}
		return result, nil
	}
}

var _ = fs.NodeStringLookuper(&Dir{})

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	child := d.Path.Sub(name)
	n, err := Follow(d.conn, child)
	switch err {
	case ErrNotExist:
		return nil, fuse.ENOENT
	case nil:
		if n.CountChildren() == 0 {
			return &File{
				fsNode: fs_node(d.conn, n, child, d),
			}, nil
		} else {
			return &Dir{
				fsNode: fs_node(d.conn, n, child, d),
			}, nil
		}
	default:
		return nil, err
	}
}

var _ = fs.NodeMkdirer(&Dir{})

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	path := d.Path.Sub(req.Name)
	if PathExists(d.conn, path) {
		return nil, fuse.EEXIST
	}
	// TODO - check permissions and return fuse.EPERM if not writable
	err := CreateOrSetBytes(d.conn, path, []byte{})
	if err != nil {
		return nil, err
	}
	n, err := Follow(d.conn, path)
	if err != nil {
		return nil, err
	}
	return &Dir{
		fsNode: fs_node(d.conn, n, path, d),
	}, nil
}

var _ = fs.NodeCreater(&Dir{})

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	path := d.Path.Sub(req.Name)
	if PathExists(d.conn, path) {
		return nil, nil, fuse.EEXIST
	}

	err := CreateOrSetBytes(d.conn, path, []byte(""))
	if err != nil {
		return nil, nil, err
	}
	n, err := d.conn.Get(path.Path())
	if err != nil {
		return nil, nil, err
	}
	f := &File{
		fsNode: fs_node(d.conn, n, path, d),
	}
	glog.Infoln(">>>> create", path, f, n)
	return f, f, nil
}

var _ = fs.NodeRemover(&Dir{})

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	path := d.Path.Sub(req.Name)
	if !PathExists(d.conn, path) {
		return fuse.ENOENT
	}
	return DeleteObject(d.conn, path)
}

var _ = fs.Node(&File{})
var _ = fs.Handle(&File{})

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := Follow(f.conn, f.Path)
	if err != nil {
		return err
	}
	a.Mode = 0644
	a.Size = uint64(len(n.Value))
	return nil
}

var _ = fs.NodeOpener(&File{})

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	glog.Infoln("Open", ctx, req, resp)

	if req.Flags.IsReadOnly() {
		// we don't need to track read-only handles
		return f, nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writers == 0 || f.node == nil {
		if n, err := Follow(f.conn, f.Path); err != nil {
			return nil, err
		} else {
			f.node = n
		}
	}

	f.writers++
	return f, nil
}

var _ = fs.HandleReleaser(&File{})

func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.writers--
	return nil
}

var _ = fs.HandleReader(&File{})

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.node == nil {
		if n, err := Follow(f.conn, f.Path); err != nil {
			return nil
		} else {
			f.node = n
		}
	}
	fuseutil.HandleRead(req, resp, f.node.Value)
	return nil
}

var _ = fs.HandleWriter(&File{})

const maxInt = int(^uint(0) >> 1)

func (f *File) write_data(input []byte, offset int64) (int, error) {
	glog.Infoln("write_data", string(input), offset)

	// expand the buffer if necessary
	newLen := offset + int64(len(input))
	if newLen > int64(maxInt) {
		return 0, fuse.Errno(syscall.EFBIG)
	}
	if newLen := int(newLen); newLen > len(f.node.Value) {
		f.node.Value = append(f.node.Value, make([]byte, newLen-len(f.node.Value))...)
	}
	n := copy(f.node.Value[offset:], input)
	err := f.node.Set(f.node.Value)
	glog.Infoln("write_data", string(f.node.Value), err)

	return n, err
}

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	glog.Infoln("Write", ctx, req, resp)
	_, err := f.write_data(req.Data, req.Offset)
	return err
}

var _ = fs.HandleFlusher(&File{})

func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	glog.Infoln("Flush", ctx, req)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writers == 0 {
		// Read-only handles also get flushes. Make sure we don't
		// overwrite valid file contents with a nil buffer.
		return nil
	}
	_, err := f.write_data(f.node.Value, 0)
	glog.Infoln("flush:", string(f.node.Value), err)
	return err
}

var _ = fs.NodeSetattrer(&File{})

func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	glog.Infoln("Setattr", ctx, req, resp)
	f.mu.Lock()
	defer f.mu.Unlock()

	if req.Valid.Size() {
		if req.Size > uint64(maxInt) {
			return fuse.Errno(syscall.EFBIG)
		}
		newLen := int(req.Size)
		switch {
		case newLen > len(f.node.Value):
			f.node.Value = append(f.node.Value, make([]byte, newLen-len(f.node.Value))...)
		case newLen < len(f.node.Value):
			f.node.Value = f.node.Value[:newLen]
		}
	}
	return nil
}
