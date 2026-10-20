package scanner

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unicode/utf16"
)

const fileIDBothDirectoryRecordSize = 104
const filetimeUnixEpoch = uint64(116444736000000000)
const filetimeTicksPerSecond = uint64(10_000_000)

type fileIDDirectoryRecord struct {
	name           string
	fileID         uint64
	logicalSize    uint64
	allocationSize uint64
	attributes     uint32
	lastWriteTime  uint64
}

func parseFileIDBothDirectoryBuffer(buffer []byte) ([]fileIDDirectoryRecord, error) {
	if len(buffer) == 0 {
		return nil, errors.New("empty directory information buffer")
	}

	var records []fileIDDirectoryRecord
	for position := 0; ; {
		remaining := len(buffer) - position
		if remaining < fileIDBothDirectoryRecordSize {
			return nil, fmt.Errorf("directory record at offset %d is truncated", position)
		}

		record := buffer[position:]
		next := int(binary.LittleEndian.Uint32(record[0:4]))
		nameLength := int(binary.LittleEndian.Uint32(record[60:64]))
		if nameLength%2 != 0 || nameLength > remaining-fileIDBothDirectoryRecordSize {
			return nil, fmt.Errorf("directory record at offset %d has invalid name length", position)
		}
		if next != 0 && (next < fileIDBothDirectoryRecordSize+nameLength || next%8 != 0 || next > remaining) {
			return nil, fmt.Errorf("directory record at offset %d has invalid next offset", position)
		}

		nameUnits := make([]uint16, nameLength/2)
		for i := range nameUnits {
			nameUnits[i] = binary.LittleEndian.Uint16(record[fileIDBothDirectoryRecordSize+i*2:])
		}
		records = append(records, fileIDDirectoryRecord{
			name:           string(utf16.Decode(nameUnits)),
			fileID:         binary.LittleEndian.Uint64(record[96:104]),
			logicalSize:    binary.LittleEndian.Uint64(record[40:48]),
			allocationSize: binary.LittleEndian.Uint64(record[48:56]),
			attributes:     binary.LittleEndian.Uint32(record[56:60]),
			lastWriteTime:  binary.LittleEndian.Uint64(record[24:32]),
		})

		if next == 0 {
			return records, nil
		}
		position += next
	}
}

func filetimeToTime(raw uint64) time.Time {
	if raw >= filetimeUnixEpoch {
		delta := raw - filetimeUnixEpoch
		return time.Unix(int64(delta/filetimeTicksPerSecond), int64(delta%filetimeTicksPerSecond)*100).UTC()
	}

	delta := filetimeUnixEpoch - raw
	seconds := int64(delta / filetimeTicksPerSecond)
	nanoseconds := int64(delta%filetimeTicksPerSecond) * 100
	if nanoseconds == 0 {
		return time.Unix(-seconds, 0).UTC()
	}
	return time.Unix(-seconds-1, int64(time.Second)-nanoseconds).UTC()
}
