//go:build !windows

package serve

func volumeSpace(string) (total, free uint64, ok bool) {
	return 0, 0, false
}

func defaultQuarantineRoots() []string {
	return nil
}
