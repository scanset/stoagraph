//go:build !unix

package oauth

// file-kw: oauth lock fallback non-unix single-process

// withLock is a no-op on platforms without flock. The gate ships as Linux containers, where the real
// implementation in lock_unix.go serializes refreshes across stag-serve and stag-proxy. On other
// platforms a single process is assumed; concurrent refreshes there could still race a rotating provider.
func (s Store) withLock(_ string, fn func() error) error { return fn() }
