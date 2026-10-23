package report

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DRAG0NM/Pare/internal/model"
)

func TestWriteJSONIncludesSchemaAndCoverageErrors(t *testing.T) {
	report := model.ScanReport{
		Schema:   model.ScanSchema,
		Complete: false,
		Errors:   []model.ScanError{{Path: `C:\restricted`, Operation: "enumerate", Code: "access_denied"}},
		StartedAt: time.Date(2026, time.October, 20, 0, 0, 0, 0,
			time.UTC),
		CompletedAt: time.Date(2026, time.October, 20, 0, 0, 1, 0, time.UTC),
	}
	var output strings.Builder
	if err := WriteJSON(&output, report); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	for _, want := range []string{`"schema":"pare.scan/1"`, `"complete":false`, `"code":"access_denied"`} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("JSON %q does not contain %q", output.String(), want)
		}
	}
}

func TestWriteJSONReturnsWriterError(t *testing.T) {
	want := errors.New("write failed")
	if err := WriteJSON(failingWriter{err: want}, model.ScanReport{Schema: model.ScanSchema}); !errors.Is(err, want) {
		t.Fatalf("WriteJSON() error = %v, want %v", err, want)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }
