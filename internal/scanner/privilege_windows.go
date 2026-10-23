//go:build windows

package scanner

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// enableBackupPrivilege turns on SeBackupPrivilege for this process when the account
// holds it, which elevated administrators do. With it, directories that grant access only
// to SYSTEM (such as System Volume Information) can be listed through the backup
// semantics the scanner already uses. It only bypasses read checks; the scanner never
// opens anything for writing. The returned function restores the previous state, and
// enabled reports whether the privilege is active during the scan. The token is shared by
// the whole process, so callers must not run scans with different expectations at once.
func enableBackupPrivilege() (restore func(), enabled bool) {
	noop := func() {}
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return noop, false
	}
	name, _ := windows.UTF16PtrFromString("SeBackupPrivilege")
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		token.Close()
		return noop, false
	}

	before := privilegeEnabled(token, luid)
	if !before {
		setPrivilege(token, luid, windows.SE_PRIVILEGE_ENABLED)
	}
	after := privilegeEnabled(token, luid)
	return func() {
		if !before && after {
			setPrivilege(token, luid, 0)
		}
		token.Close()
	}, after
}

// setPrivilege ignores the result: AdjustTokenPrivileges reports a privilege the account
// does not hold only through the last error, so callers re-read the token instead.
func setPrivilege(token windows.Token, luid windows.LUID, attributes uint32) {
	state := windows.Tokenprivileges{PrivilegeCount: 1}
	state.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: attributes}
	_ = windows.AdjustTokenPrivileges(token, false, &state, 0, nil, nil)
}

func privilegeEnabled(token windows.Token, luid windows.LUID) bool {
	var size uint32
	_ = windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &size)
	if size == 0 {
		return false
	}
	buffer := make([]byte, size)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buffer[0], size, &size); err != nil {
		return false
	}
	list := (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0]))
	for _, privilege := range unsafe.Slice(&list.Privileges[0], list.PrivilegeCount) {
		if privilege.Luid == luid {
			return privilege.Attributes&windows.SE_PRIVILEGE_ENABLED != 0
		}
	}
	return false
}
