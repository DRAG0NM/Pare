//go:build windows

package winexec

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A command the AI wrote is not validated by Pare, so when Pare itself runs elevated it
// must not inherit that. normalUserToken derives a token from Pare's own: the Windows
// "normal user" safer level turns Administrators into a deny-only group and drops the
// admin privileges, and the integrity level is lowered to medium. A restricted copy of
// the caller's own token needs no extra privilege for CreateProcessAsUser.

const (
	saferScopeUser       = 2
	saferLevelNormalUser = 0x20000
	saferLevelOpen       = 1
	seGroupIntegrity     = 0x20
)

var (
	advapi32                   = windows.NewLazySystemDLL("advapi32.dll")
	saferCreateLevel           = advapi32.NewProc("SaferCreateLevel")
	saferComputeTokenFromLevel = advapi32.NewProc("SaferComputeTokenFromLevel")
	saferCloseLevel            = advapi32.NewProc("SaferCloseLevel")
)

// elevated reports whether this process runs with an elevated token.
func elevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// normalUserToken returns a primary token without administrator rights. The caller
// closes it.
func normalUserToken() (windows.Token, error) {
	var level uintptr
	if ok, _, err := saferCreateLevel.Call(saferScopeUser, saferLevelNormalUser, saferLevelOpen, uintptr(unsafe.Pointer(&level)), 0); ok == 0 {
		return 0, fmt.Errorf("SaferCreateLevel: %w", err)
	}
	defer saferCloseLevel.Call(level)

	var token windows.Token
	if ok, _, err := saferComputeTokenFromLevel.Call(level, 0, uintptr(unsafe.Pointer(&token)), 0, 0); ok == 0 {
		return 0, fmt.Errorf("SaferComputeTokenFromLevel: %w", err)
	}

	medium, err := windows.CreateWellKnownSid(windows.WinMediumLabelSid)
	if err != nil {
		token.Close()
		return 0, err
	}
	label := windows.Tokenmandatorylabel{Label: windows.SIDAndAttributes{Sid: medium, Attributes: seGroupIntegrity}}
	if err := windows.SetTokenInformation(token, windows.TokenIntegrityLevel, (*byte)(unsafe.Pointer(&label)), uint32(unsafe.Sizeof(label))+windows.GetLengthSid(medium)); err != nil {
		token.Close()
		return 0, fmt.Errorf("lower integrity level: %w", err)
	}
	return token, nil
}
