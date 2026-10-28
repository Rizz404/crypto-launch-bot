package catalyst

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const DailyInferenceLimit = 12

// Reserve before I/O; crashes, timeouts and provider failures consume a slot.
// Exclusive files enforce the cap across processes and restarts on this archive.
func (a Archive) reserveInference(now time.Time) error {
	day := now.In(time.FixedZone("WIB", 7*60*60)).Format("2006-01-02")
	dir := filepath.Join(a.Dir, "daily-budget", day)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	for n := range DailyInferenceLimit {
		f, err := os.OpenFile(filepath.Join(dir, fmt.Sprint(n)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err = f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		for _, path := range []string{dir, filepath.Dir(dir), a.Dir} {
			d, err := os.Open(path)
			if err != nil {
				return err
			}
			err = d.Sync()
			_ = d.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	return errors.New("daily AI budget exhausted: 12 calls per WIB day")
}
