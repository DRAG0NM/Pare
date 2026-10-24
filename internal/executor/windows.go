//go:build windows

package executor

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/scanner"
)

func init() {
	openLocked = openWindowsLocked
	hasReparseAncestor = windowsHasReparseAncestor
}

type windowsFile struct{ handle windows.Handle }

// openWindowsLocked opens path with no sharing, so no other process can read, write,
// rename or delete it while Pare holds the handle. Reparse points are opened themselves,
// never followed.
func openWindowsLocked(path string) (lockedFile, error) {
	apiPath, err := scanner.WindowsPath(path)
	if err != nil {
		return nil, err
	}
	pointer, err := windows.UTF16PtrFromString(apiPath)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		pointer,
		windows.DELETE|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		0,
		nil,
		windows.OPEN_EXISTING,
		// Backup semantics are required to open directories; reparse points are opened
		// themselves, never followed.
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	switch {
	case errors.Is(err, windows.ERROR_SHARING_VIOLATION):
		return nil, errInUse
	case errors.Is(err, windows.ERROR_FILE_NOT_FOUND), errors.Is(err, windows.ERROR_PATH_NOT_FOUND):
		return nil, errMissing
	case err != nil:
		return nil, err
	}
	return windowsFile{handle: handle}, nil
}

func (f windowsFile) Info() (fileInfo, error) {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(f.handle, &info); err != nil {
		return fileInfo{}, err
	}
	return fileInfo{
		FileID:     fmt.Sprintf("%016x", uint64(info.FileIndexHigh)<<32|uint64(info.FileIndexLow)),
		Size:       uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow),
		LastWrite:  time.Unix(0, info.LastWriteTime.Nanoseconds()),
		Attributes: info.FileAttributes,
		Reparse:    info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0,
		Directory:  info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0,
	}, nil
}

type fileRenameInfo struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

// Rename moves the open file to destination on the same volume. It never replaces an
// existing file and never copies across volumes.
func (f windowsFile) Rename(destination string) error {
	apiPath, err := scanner.WindowsPath(destination)
	if err != nil {
		return err
	}
	name, err := windows.UTF16FromString(apiPath)
	if err != nil {
		return err
	}
	name = name[:len(name)-1]
	var header fileRenameInfo
	offset := int(unsafe.Offsetof(header.FileName))
	buffer := make([]byte, offset+len(name)*2+2)
	info := (*fileRenameInfo)(unsafe.Pointer(&buffer[0]))
	info.FileNameLength = uint32(len(name) * 2)
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(&buffer[offset])), len(name)), name)

	err = windows.SetFileInformationByHandle(f.handle, windows.FileRenameInfo, &buffer[0], uint32(len(buffer)))
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
		return errExists
	}
	// Windows refuses to rename a directory while any file inside is open.
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return fmt.Errorf("%w: %v", errInUse, err)
	}
	return err
}

func (f windowsFile) Close() error {
	return windows.CloseHandle(f.handle)
}

// windowsHasReparseAncestor reports whether any existing directory between the volume
// root and path's parent is a reparse point.
func windowsHasReparseAncestor(path string) (bool, error) {
	volume := filepath.VolumeName(path)
	rest := strings.TrimPrefix(filepath.Dir(path), volume)
	current := volume + `\`
	for _, segment := range strings.FieldsFunc(rest, func(r rune) bool { return r == '\\' || r == '/' }) {
		current = filepath.Join(current, segment)
		apiPath, err := scanner.WindowsPath(current)
		if err != nil {
			return false, err
		}
		pointer, err := windows.UTF16PtrFromString(apiPath)
		if err != nil {
			return false, err
		}
		attributes, err := windows.GetFileAttributes(pointer)
		if err != nil {
			return false, err
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return true, nil
		}
	}
	return false, nil
}
