//go:build windows

package source

type Index struct {
	*portableIndex
}

func New(dataDir, snapshot string) *Index {
	return &Index{portableIndex: newPortableIndex(dataDir, snapshot)}
}
