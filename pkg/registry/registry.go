package registry

type Path string

func (p Path) String() string {
	return string(p)
}

func (p Path) Valid() bool {
	return len(p) > 0
}
