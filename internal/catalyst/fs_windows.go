package catalyst

import (
	"os"

	"golang.org/x/sys/windows"
)

// LockFile takes an exclusive lock over the whole file; wait=false fails instead of blocking.
func LockFile(f *os.File, wait bool) error {
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK)
	if !wait {
		flags |= windows.LOCKFILE_FAIL_IMMEDIATELY
	}
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, ^uint32(0), ^uint32(0), new(windows.Overlapped))
}

func UnlockFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, ^uint32(0), ^uint32(0), new(windows.Overlapped))
}

// SyncDir is a no-op: Windows cannot fsync a directory handle, and NTFS journals
// directory metadata itself. File contents are still synced by the callers.
func SyncDir(path string) error {
	_, err := os.Stat(path)
	return err
}

func diskSpace(dir string) (available, total int64, err error) {
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, 0, err
	}
	var free, size, totalFree uint64
	if err = windows.GetDiskFreeSpaceEx(path, &free, &size, &totalFree); err != nil {
		return 0, 0, err
	}
	return int64(free), int64(size), nil
}
