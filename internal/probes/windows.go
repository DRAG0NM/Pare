//go:build windows

package probes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/model"
	"github.com/DRAG0NM/Pare/internal/scanner"
)

type windowsProber struct{}

func NewWindowsProber() Prober {
	return windowsProber{}
}

// Probe opens the file for reading with no sharing allowed. Success shows that no other
// process holds it open; the same handle is used to compare identity and metadata with
// the scan. Nothing is read from or written to the file.
func (windowsProber) Probe(ctx context.Context, node model.Node) []model.Evidence {
	if err := ctx.Err(); err != nil {
		return unknown("probe cancelled")
	}
	apiPath, err := scanner.WindowsPath(node.Path)
	if err != nil {
		return unknown(err.Error())
	}
	pathPointer, err := windows.UTF16PtrFromString(apiPath)
	if err != nil {
		return unknown(err.Error())
	}
	handle, err := windows.CreateFile(
		pathPointer,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	switch {
	case errors.Is(err, windows.ERROR_SHARING_VIOLATION):
		return []model.Evidence{
			{Kind: KindUnchanged, Status: model.EvidenceUnknown, Detail: "file is open in another process"},
			{Kind: KindUnlocked, Status: model.EvidenceFail, Detail: "file is open in another process"},
		}
	case errors.Is(err, windows.ERROR_FILE_NOT_FOUND), errors.Is(err, windows.ERROR_PATH_NOT_FOUND):
		return []model.Evidence{
			{Kind: KindUnchanged, Status: model.EvidenceFail, Detail: "file no longer exists"},
			{Kind: KindUnlocked, Status: model.EvidenceUnknown, Detail: "file no longer exists"},
		}
	case err != nil:
		return []model.Evidence{
			{Kind: KindUnchanged, Status: model.EvidenceError, Detail: err.Error()},
			{Kind: KindUnlocked, Status: model.EvidenceError, Detail: err.Error()},
		}
	}
	defer windows.CloseHandle(handle)

	unlocked := model.Evidence{Kind: KindUnlocked, Status: model.EvidencePass, Detail: "exclusive read open succeeded"}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return []model.Evidence{{Kind: KindUnchanged, Status: model.EvidenceError, Detail: err.Error()}, unlocked}
	}
	return []model.Evidence{compare(node, info), unlocked}
}

// compare does not check the volume serial: GetFileInformationByHandle can report a
// different serial for a directory than for the files inside it (seen on an EFS-encrypted
// AppData\Local), so the scan's root serial is not a reliable per-file value.
func compare(node model.Node, info windows.ByHandleFileInformation) model.Evidence {
	var changes []string
	fileID := fmt.Sprintf("%016x", uint64(info.FileIndexHigh)<<32|uint64(info.FileIndexLow))
	if fileID != node.FileID {
		changes = append(changes, "file_id")
	}
	if uint64(info.FileSizeHigh)<<32|uint64(info.FileSizeLow) != uint64(node.LogicalSize) {
		changes = append(changes, "size")
	}
	if !time.Unix(0, info.LastWriteTime.Nanoseconds()).Equal(node.LastWriteTime) {
		changes = append(changes, "last_write_time")
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		changes = append(changes, "reparse")
	}
	if len(changes) != 0 {
		return model.Evidence{Kind: KindUnchanged, Status: model.EvidenceFail, Detail: "changed since scan: " + strings.Join(changes, ", ")}
	}
	return model.Evidence{Kind: KindUnchanged, Status: model.EvidencePass, Detail: "file ID, size and last write time match the scan"}
}
