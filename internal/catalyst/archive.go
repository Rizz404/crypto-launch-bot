package catalyst

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// Archive stores immutable, content-addressed research records, never funds.
// Atomic hard links prevent overwrites under concurrent writes.
type Archive struct {
	Dir      string
	MaxBytes int64
}

const maxArchiveBytes = MaxDocumentBytes * 8
const DefaultArchiveBytes int64 = 2 << 30

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
	if err = os.MkdirAll(a.Dir, 0700); err != nil {
		return "", err
	}
	lock, err := os.OpenFile(filepath.Join(a.Dir, ".write-lock"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer func() { _ = lock.Close() }()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return "", err
	}
	defer func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }()
	dir := filepath.Join(a.Dir, kind)
	target := filepath.Join(dir, id+".json")
	if st, statErr := os.Stat(target); statErr == nil && st.Size() > maxArchiveBytes {
		return "", errors.New("existing archive record exceeds size limit")
	}
	if old, readErr := os.ReadFile(target); readErr == nil {
		if !bytes.Equal(old, b) {
			return "", errors.New("existing archive record corrupted")
		}
		return id, nil
	} else if !errors.Is(readErr, fs.ErrNotExist) {
		return "", readErr
	}
	if err = a.capacity(int64(len(b))); err != nil {
		return "", err
	}
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
	if err = os.Link(f.Name(), target); err != nil && !errors.Is(err, fs.ErrExist) {
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
	root, err := os.Open(a.Dir)
	if err != nil {
		return "", err
	}
	err = root.Sync()
	_ = root.Close()
	if err != nil {
		return "", err
	}
	return id, nil
}

// Preserve evidence; reaching the quota stops new research instead of deleting
// history. Leave filesystem headroom for the rest of the shared host.
func (a Archive) capacity(extra int64) error {
	limit := a.MaxBytes
	if limit == 0 {
		limit = DefaultArchiveBytes
	}
	if limit < maxArchiveBytes {
		return errors.New("archive quota below record limit")
	}
	var used int64
	err := filepath.WalkDir(a.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			used += info.Size()
		}
		if used+extra > limit {
			return errors.New("archive disk quota reached; new research paused")
		}
		return nil
	})
	if err != nil {
		return err
	}
	var st syscall.Statfs_t
	if err = syscall.Statfs(a.Dir, &st); err != nil {
		return err
	}
	reserve := min(int64(1<<30), int64(st.Blocks)*st.Bsize/10)
	if int64(st.Bavail)*st.Bsize < reserve+extra {
		return errors.New("archive filesystem headroom exhausted")
	}
	return nil
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
