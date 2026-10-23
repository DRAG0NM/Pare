package report

import (
	"encoding/json"
	"io"
)

// WriteJSON writes a scan report or plan as one JSON document.
func WriteJSON(output io.Writer, document any) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(document)
}
