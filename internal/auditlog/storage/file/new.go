package file

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"github.com/containerssh/containerssh/config"
	"github.com/containerssh/containerssh/internal/auditlog/storage"

	"github.com/containerssh/containerssh/log"
)

// NewStorage Create a file storage that stores testdata in a local directory. The file storage cannot store metadata.
func NewStorage(cfg config.AuditLogFileConfig, _ log.Logger) (storage.ReadWriteStorage, error) {
	if cfg.Directory == "" {
		return nil, fmt.Errorf("invalid audit log directory")
	}
	stat, err := os.Stat(cfg.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to access audit log directory %s (%w)", cfg.Directory, err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("specified audit log directory is not a directory %s (%w)", cfg.Directory, err)
	}
	err = ioutil.WriteFile(path.Join(cfg.Directory, ".accesstest"), []byte{}, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create file in audit log directory %s (%w)", cfg.Directory, err)
	}
	return &fileStorage{
		directory: cfg.Directory,
		wg:        &sync.WaitGroup{},
	}, nil
}
