//go:build !windows

package advisor

func protect([]byte) ([]byte, error) {
	return nil, ErrNoProtection
}

func unprotect([]byte) ([]byte, error) {
	return nil, ErrNoProtection
}
