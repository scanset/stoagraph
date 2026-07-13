//go:build unix

package oauth

// file-kw: oauth cross-process file lock flock refresh serialization rotation

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// withLock runs fn holding an EXCLUSIVE, cross-PROCESS lock on this server's token file.
//
// It has to be cross-process, not just a mutex: stag-proxy (refreshing at connect) and stag-serve
// (refreshing during discovery) are separate containers sharing data/oauth/ on one volume. A refresh
// token is SINGLE-USE under rotation, so two processes refreshing at once would spend the same token —
// one wins, the other gets invalid_grant, and a revoked token can end up persisted, locking the operator
// out of a server they authorized.
//
// flock is released by the kernel if the holder dies, so a crashed process cannot wedge the lock.
func (s Store) withLock(server string, fn func() error) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	lockPath := filepath.Join(s.Dir, safeName(server)+".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("oauth: open lock: %w", err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("oauth: lock %s: %w", lockPath, err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}
