package maintenance

import (
	"path/filepath"
	"testing"
)

func TestDataLockPreventsConcurrentRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coc.db")
	first, err := AcquireDataLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if second, err := AcquireDataLock(path); err == nil {
		_ = second.Close()
		t.Fatal("second data lock unexpectedly succeeded")
	}
}
