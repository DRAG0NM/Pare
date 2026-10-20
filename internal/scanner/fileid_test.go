package scanner

import (
	"encoding/binary"
	"testing"
	"time"
	"unicode/utf16"
)

func TestFiletimeToTimePreservesDatesBeyondUnixNanosecondRange(t *testing.T) {
	const raw = uint64(132539328000000000)
	got := filetimeToTime(raw)
	want := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("filetimeToTime(%d) = %s, want %s", raw, got, want)
	}

	want = time.Date(2570, time.January, 1, 0, 0, 0, 0, time.UTC)
	after2262 := uint64(want.Unix())*10_000_000 + 116_444_736_000_000_000
	got = filetimeToTime(after2262)
	if !got.Equal(want) {
		t.Fatalf("filetimeToTime(%d) = %s, want %s", after2262, got, want)
	}
}

func TestParseFileIDBothDirectoryBufferPreservesHardLinkPaths(t *testing.T) {
	buffer := appendDirectoryRecord(nil, "first.bin", 42, 12, 16, 0x20, 0)
	buffer = appendDirectoryRecord(buffer, "second.bin", 42, 12, 16, 0x20, 0)

	records, err := parseFileIDBothDirectoryBuffer(buffer)
	if err != nil {
		t.Fatalf("parseFileIDBothDirectoryBuffer() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("record count = %d, want both directory entries", len(records))
	}
	if records[0].fileID != 42 || records[1].fileID != 42 {
		t.Fatalf("file IDs = %d, %d, want shared ID 42", records[0].fileID, records[1].fileID)
	}
	if records[0].name != "first.bin" || records[1].name != "second.bin" {
		t.Fatalf("names = %q, %q, want both hard-link paths", records[0].name, records[1].name)
	}
}

func TestParseFileIDBothDirectoryBufferRejectsInvalidNextOffset(t *testing.T) {
	buffer := appendDirectoryRecord(nil, "first.bin", 42, 12, 16, 0x20, 0)
	binary.LittleEndian.PutUint32(buffer[0:4], 8)

	if _, err := parseFileIDBothDirectoryBuffer(buffer); err == nil {
		t.Fatal("parseFileIDBothDirectoryBuffer() error = nil, want invalid offset error")
	}
}

func TestParseFileIDBothDirectoryBufferRejectsTruncatedName(t *testing.T) {
	buffer := appendDirectoryRecord(nil, "first.bin", 42, 12, 16, 0x20, 0)
	binary.LittleEndian.PutUint32(buffer[60:64], uint32(len(buffer)))

	if _, err := parseFileIDBothDirectoryBuffer(buffer); err == nil {
		t.Fatal("parseFileIDBothDirectoryBuffer() error = nil, want truncated-name error")
	}
}

func appendDirectoryRecord(buffer []byte, name string, fileID uint64, size, allocation uint64, attributes uint32, next uint32) []byte {
	nameBytes := make([]byte, len(utf16.Encode([]rune(name)))*2)
	for i, unit := range utf16.Encode([]rune(name)) {
		binary.LittleEndian.PutUint16(nameBytes[i*2:], unit)
	}
	length := 104 + len(nameBytes)
	recordLength := (length + 7) &^ 7
	if next == 0 && len(buffer) != 0 {
		start := lastRecordStart(buffer)
		binary.LittleEndian.PutUint32(buffer[start:start+4], uint32(len(buffer)-start))
	}
	start := len(buffer)
	buffer = append(buffer, make([]byte, recordLength)...)
	binary.LittleEndian.PutUint32(buffer[start:start+4], next)
	binary.LittleEndian.PutUint64(buffer[start+24:start+32], 133751808000000000)
	binary.LittleEndian.PutUint64(buffer[start+40:start+48], size)
	binary.LittleEndian.PutUint64(buffer[start+48:start+56], allocation)
	binary.LittleEndian.PutUint32(buffer[start+56:start+60], attributes)
	binary.LittleEndian.PutUint32(buffer[start+60:start+64], uint32(len(nameBytes)))
	binary.LittleEndian.PutUint64(buffer[start+96:start+104], fileID)
	copy(buffer[start+104:start+length], nameBytes)
	return buffer
}

func lastRecordStart(buffer []byte) int {
	position := 0
	for {
		if position+4 > len(buffer) {
			panic("invalid test directory-record buffer")
		}
		next := int(binary.LittleEndian.Uint32(buffer[position : position+4]))
		if next == 0 {
			return position
		}
		if next < 104 || position+next >= len(buffer) {
			panic("invalid test directory-record offset")
		}
		position += next
	}
}
