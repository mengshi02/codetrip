package search

import (
	"os"
	"path/filepath"

	"github.com/mengshi02/codetrip/internal/store"
)

func mkTmpDir(prefix string) (string, error) {
	return os.MkdirTemp("", prefix)
}

func defaultCfg(dir string) store.Config {
	return store.DefaultConfig(filepath.Join(dir, "db"))
}

func openStore(cfg store.Config) (*store.Store, error) {
	return store.Open(cfg)
}

func rmDir(dir string) {
	os.RemoveAll(dir)
}