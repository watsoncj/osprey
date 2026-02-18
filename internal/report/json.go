package report

import (
	"encoding/json"
	"io"

	"github.com/browser-forensics/browser-forensics/internal/model"
)

type JSONReporter struct{}

func (j *JSONReporter) Write(w io.Writer, rr model.RunReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rr)
}
