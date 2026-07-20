package semantic

import (
	"os"
	"path/filepath"

	store "github.com/mengshi02/codetrip/internal/storage"
)

func mkTmpDir(prefix string) (string, error) { return os.MkdirTemp("", prefix) }

func defaultCfg(dir string) store.Config { return store.DefaultConfig(filepath.Join(dir, "db")) }

func openStore(cfg store.Config) (*store.Store, error) { return store.Open(cfg) }

func rmDir(dir string) { _ = os.RemoveAll(dir) }
