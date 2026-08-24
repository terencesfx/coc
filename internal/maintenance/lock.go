package maintenance

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type DataLock struct{ file *os.File }

func AcquireDataLock(databasePath string) (*DataLock, error) {
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(databasePath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("data directory is in use: %w", err)
	}
	return &DataLock{file: file}, nil
}

func (lock *DataLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	_ = syscall.Flock(int(lock.file.Fd()), syscall.LOCK_UN)
	return lock.file.Close()
}
