package report

import (
	"io"

	"github.com/watsoncj/osprey/internal/model"
)

// Reporter writes a RunReport to the given writer.
type Reporter interface {
	Write(w io.Writer, rr model.RunReport) error
}

// Get returns a reporter for the given format name.
func Get(format string) Reporter {
	switch format {
	case "json":
		return &JSONReporter{}
	default:
		return &TextReporter{}
	}
}
