//go:build windows

package scanner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/DRAG0NM/Pare/internal/model"
)

const directoryBufferSize = 64 * 1024

type windowsScanner struct {
	options Options
}

func NewWindowsScanner() Scanner {
	return NewWindowsScannerWithOptions(Options{})
}

func NewWindowsScannerWithOptions(options Options) Scanner {
	if options.Workers < 1 {
		options.Workers = DefaultWorkers
	}
	if options.ProgressInterval <= 0 {
		options.ProgressInterval = DefaultProgressInterval
	}
	return windowsScanner{options: options}
}

// Scan reads directories with a fixed pool of workers sharing one queue. Nodes and
// errors are sorted by path at the end, so the report does not depend on scheduling.
func (s windowsScanner) Scan(ctx context.Context, root string) (model.ScanReport, error) {
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
	volume := report.Volume

	var (
		mu       sync.Mutex
		wake     = sync.NewCond(&mu)
		queue    = []string{absoluteRoot}
		pending  = 1
		progress Progress
	)
	worker := func() {
		mu.Lock()
		defer mu.Unlock()
		for {
			for len(queue) == 0 && pending > 0 {
				wake.Wait()
			}
			if pending == 0 {
				return
			}
			directory := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			progress.Current = directory
			mu.Unlock()

			var records []fileIDDirectoryRecord
			var serial uint32
			var readErr error
			if ctx.Err() == nil {
				records, serial, readErr = enumerateWindowsDirectory(ctx, directory)
			}
			nodes, children := directoryNodes(directory, records)

			mu.Lock()
			switch {
			case ctx.Err() != nil:
				report.Complete = false
				children = nil
			case readErr != nil:
				report.Complete = false
				report.Errors = append(report.Errors, model.ScanError{
					Path: directory, Operation: "enumerate", Code: windowsErrorCode(readErr),
				})
				nodes, children = nil, nil
			default:
				if directory == absoluteRoot {
					report.Volume = fmt.Sprintf("%s:%08x", volume, serial)
				}
				report.Nodes = append(report.Nodes, nodes...)
			}
			progress.Entries = len(report.Nodes)
			progress.Directories++
			queue = append(queue, children...)
			pending += len(children) - 1
			if pending == 0 || len(children) > 0 {
				wake.Broadcast()
			}
		}
	}

	done := make(chan struct{})
	reported := make(chan struct{})
	go func() {
		defer close(reported)
		if s.options.Progress == nil {
			<-done
			return
		}
		ticker := time.NewTicker(s.options.ProgressInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				mu.Lock()
				snapshot := progress
				mu.Unlock()
				s.options.Progress(snapshot)
			case <-done:
				mu.Lock()
				snapshot := progress
				mu.Unlock()
				snapshot.Current = ""
				snapshot.Done = true
				s.options.Progress(snapshot)
				return
			}
		}
	}()

	var workers sync.WaitGroup
	for range s.options.Workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			worker()
		}()
	}
	workers.Wait()
	close(done)
	<-reported

	sort.Slice(report.Nodes, func(i, j int) bool { return report.Nodes[i].Path < report.Nodes[j].Path })
	sort.Slice(report.Errors, func(i, j int) bool { return report.Errors[i].Path < report.Errors[j].Path })
	report.CompletedAt = time.Now().UTC()
	if err := ctx.Err(); err != nil {
		report.Complete = false
		return report, err
	}
	return report, nil
}

func directoryNodes(directory string, records []fileIDDirectoryRecord) ([]model.Node, []string) {
	nodes := make([]model.Node, 0, len(records))
	var children []string
	for _, record := range records {
		if record.name == "." || record.name == ".." {
			continue
		}
		path := filepath.Join(directory, record.name)
		kind := "file"
		if record.attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			kind = "reparse"
		} else if record.attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
			kind = "directory"
			children = append(children, path)
		}

		fileID := ""
		if record.fileID != 0 {
			fileID = fmt.Sprintf("%016x", record.fileID)
		}
		nodes = append(nodes, model.Node{
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
	return nodes, children
}

func enumerateWindowsDirectory(ctx context.Context, path string) ([]fileIDDirectoryRecord, uint32, error) {
	apiPath, err := WindowsPath(path)
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

// WindowsPath converts path to an absolute extended-length path for Win32 calls.
func WindowsPath(path string) (string, error) {
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
