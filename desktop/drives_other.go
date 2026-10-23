//go:build !windows

package main

type Drive struct {
	Root  string `json:"root"`
	Label string `json:"label"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

func (a *App) Drives() []Drive {
	return nil
}

func (a *App) Elevated() bool {
	return false
}

func volumeSpace(string) (total, free uint64, ok bool) {
	return 0, 0, false
}
