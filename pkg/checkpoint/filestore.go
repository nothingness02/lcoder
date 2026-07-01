package checkpoint

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileStore is a Store implementation that persists checkpoints as JSON files
// on the local filesystem.
type FileStore struct {
	Dir string
}

// NewFileStore creates a FileStore that writes checkpoints into dir.
func NewFileStore(dir string) *FileStore {
	return &FileStore{Dir: dir}
}

const checkpointSuffix = ".checkpoint.json"

// Save persists cp to a file named after id inside the store directory.
func (fs *FileStore) Save(id string, cp *Checkpoint) error {
	if err := os.MkdirAll(fs.Dir, 0o755); err != nil {
		return err
	}
	data, err := cp.MarshalJSON()
	if err != nil {
		return err
	}
	path := filepath.Join(fs.Dir, sanitize(id)+checkpointSuffix)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return os.Chmod(path, 0o644)
}

// Load reads and unmarshals the checkpoint stored under id.
func (fs *FileStore) Load(id string) (*Checkpoint, error) {
	path := filepath.Join(fs.Dir, sanitize(id)+checkpointSuffix)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cp := &Checkpoint{}
	if err := cp.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return cp, nil
}

// List returns the identifiers of all checkpoints in the store, sorted.
func (fs *FileStore) List() ([]string, error) {
	entries, err := os.ReadDir(fs.Dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, checkpointSuffix) {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, checkpointSuffix))
	}
	sort.Strings(ids)
	return ids, nil
}

// Delete removes the checkpoint stored under id.
func (fs *FileStore) Delete(id string) error {
	path := filepath.Join(fs.Dir, sanitize(id)+checkpointSuffix)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func sanitize(id string) string {
	return strings.ReplaceAll(id, string(filepath.Separator), "_")
}
