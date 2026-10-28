package catalyst

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Archive stores immutable, content-addressed research records, never funds.
// Atomic hard links prevent overwrites under concurrent writes.
type Archive struct{ Dir string }

const maxArchiveBytes = MaxDocumentBytes * 8

func (a Archive) Put(kind string, value any) (string, error) {
	if !identifier.MatchString(kind) || a.Dir == "" {
		return "", errors.New("invalid archive location or kind")
	}
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if len(b) > maxArchiveBytes {
		return "", errors.New("archive record exceeds size limit")
	}
	id := Hash(value)
	dir := filepath.Join(a.Dir, kind)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".pending-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err = os.Link(f.Name(), filepath.Join(dir, id+".json")); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	d, err := os.Open(dir)
	if err != nil {
		return "", err
	}
	defer func() { _ = d.Close() }()
	if err = d.Sync(); err != nil {
		return "", err
	}
	return id, nil
}

func (a Archive) Read(kind, id string, out any) error {
	if !identifier.MatchString(kind) || len(id) != 64 || strings.Trim(id, "0123456789abcdef") != "" {
		return errors.New("invalid archive key")
	}
	f, err := os.Open(filepath.Join(a.Dir, kind, id+".json"))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err = Decode(f, out, maxArchiveBytes); err != nil {
		return err
	}
	if Hash(out) != id {
		return errors.New("archive checksum mismatch")
	}
	return nil
}

func (a Archive) IDs(kind string) ([]string, error) {
	if !identifier.MatchString(kind) {
		return nil, errors.New("invalid archive kind")
	}
	entries, err := os.ReadDir(filepath.Join(a.Dir, kind))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".json")
		if !e.IsDir() && len(name) == 64 && strings.Trim(name, "0123456789abcdef") == "" {
			ids = append(ids, name)
		}
	}
	slices.Sort(ids)
	return ids, nil
}
