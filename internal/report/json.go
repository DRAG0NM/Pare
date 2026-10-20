package report

import (
	"encoding/json"
	"io"

	"github.com/DRAG0NM/Pare/internal/model"
)

func WriteJSON(output io.Writer, scan model.ScanReport) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(scan)
}
