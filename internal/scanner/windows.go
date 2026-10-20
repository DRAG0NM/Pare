//go:build windows

package scanner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/model"
)

const directoryBufferSize = 64 * 1024

type windowsScanner struct{}

func NewWindowsScanner() Scanner {
	return windowsScanner{}
}

func (windowsScanner) Scan(ctx context.Context, root string) (model.ScanReport, error) {
	started := time.Now().UTC()
	report := model.ScanReport{
		Schema:    model.ScanSchema,
		StartedAt: started,
		Complete:  true,
		Nodes:     []model.Node{},
		Errors:    []model.ScanError{},
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}
	report.Volume = filepath.VolumeName(absoluteRoot)
	if report.Volume == "" {
		return report, fmt.Errorf("scan root has no volume: %q", root)
	}

	pending := []string{absoluteRoot}
	for len(pending) != 0 {
		if err := ctx.Err(); err != nil {
			report.Complete = false
			report.CompletedAt = time.Now().UTC()
			return report, err
		}
		directory := pending[0]
		pending = pending[1:]
		records, serial, err := enumerateWindowsDirectory(ctx, directory)
		if err != nil {
			if ctx.Err() != nil {
				report.Complete = false
				report.CompletedAt = time.Now().UTC()
				return report, ctx.Err()
			}
			report.Complete = false
			report.Errors = append(report.Errors, model.ScanError{
				Path: directory, Operation: "enumerate", Code: windowsErrorCode(err),
			})
			continue
		}
		if directory == absoluteRoot {
			report.Volume = fmt.Sprintf("%s:%08x", report.Volume, serial)
		}

		for _, record := range records {
			if err := ctx.Err(); err != nil {
				report.Complete = false
				report.CompletedAt = time.Now().UTC()
				return report, err
			}
			if record.name == "." || record.name == ".." {
				continue
			}

			path := filepath.Join(directory, record.name)
			kind := "file"
			if record.attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				kind = "reparse"
			} else if record.attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
				kind = "directory"
				pending = append(pending, path)
			}

			fileID := ""
			if record.fileID != 0 {
				fileID = fmt.Sprintf("%016x", record.fileID)
			}
			report.Nodes = append(report.Nodes, model.Node{
				Path:           path,
				ParentPath:     directory,
				Kind:           kind,
				FileID:         fileID,
				LogicalSize:    model.ByteCount(record.logicalSize),
				AllocationSize: model.ByteCount(record.allocationSize),
				Attributes:     record.attributes,
				LastWriteTime:  filetimeToTime(record.lastWriteTime),
			})
		}
	}

	report.CompletedAt = time.Now().UTC()
	return report, nil
}

func enumerateWindowsDirectory(ctx context.Context, path string) ([]fileIDDirectoryRecord, uint32, error) {
	apiPath, err := windowsPath(path)
	if err != nil {
		return nil, 0, err
	}
	pathPointer, err := windows.UTF16PtrFromString(apiPath)
	if err != nil {
		return nil, 0, err
	}
	handle, err := windows.CreateFile(
		pathPointer,
		windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, 0, err
	}
	defer windows.CloseHandle(handle)

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, 0, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, info.VolumeSerialNumber, windows.ERROR_CANT_ACCESS_FILE
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return nil, info.VolumeSerialNumber, windows.ERROR_DIRECTORY
	}

	buffer := make([]byte, directoryBufferSize)
	var records []fileIDDirectoryRecord
	for {
		if err := ctx.Err(); err != nil {
			return records, info.VolumeSerialNumber, err
		}
		err := windows.GetFileInformationByHandleEx(
			handle,
			windows.FileIdBothDirectoryInfo,
			&buffer[0],
			uint32(len(buffer)),
		)
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return records, info.VolumeSerialNumber, nil
		}
		if err != nil {
			return records, info.VolumeSerialNumber, err
		}
		batch, err := parseFileIDBothDirectoryBuffer(buffer)
		if err != nil {
			return records, info.VolumeSerialNumber, err
		}
		records = append(records, batch...)
	}
}

func windowsPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(absolute, `\\?\`) {
		return absolute, nil
	}
	if strings.HasPrefix(absolute, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(absolute, `\\`), nil
	}
	return `\\?\` + absolute, nil
}

func windowsErrorCode(err error) string {
	var code syscall.Errno
	if !errors.As(err, &code) {
		return "io_error"
	}
	switch code {
	case windows.ERROR_ACCESS_DENIED:
		return "access_denied"
	case windows.ERROR_FILE_NOT_FOUND, windows.ERROR_PATH_NOT_FOUND, windows.ERROR_DELETE_PENDING:
		return "changed_entry"
	case windows.ERROR_INVALID_FUNCTION, windows.ERROR_NOT_SUPPORTED, windows.ERROR_INVALID_PARAMETER:
		return "unsupported_information_class"
	default:
		return "io_error"
	}
}
