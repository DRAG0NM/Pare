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
