package ingest

import (
	"github.com/watsoncj/osprey/internal/decoder"
	"github.com/watsoncj/osprey/internal/flagging"
	"github.com/watsoncj/osprey/internal/model"
)

// Pipeline enriches raw visit data with decoded URLs and flags.
type Pipeline struct {
	decoders *decoder.Registry
	flagger  *flagging.Flagger
}

// New creates a Pipeline with the default decoder registry and flagger.
func New() *Pipeline {
	return &Pipeline{
		decoders: decoder.DefaultRegistry(),
		flagger:  flagging.DefaultFlagger(),
	}
}

// ProcessVisits converts raw visits into enriched visits with decoded URLs and flags.
func (p *Pipeline) ProcessVisits(raw []model.RawVisit) []model.Visit {
	visits := make([]model.Visit, len(raw))
	for i, r := range raw {
		v := model.Visit{
			Time:    r.Time,
			URL:     r.URL,
			Title:   r.Title,
			Browser: r.Browser,
			DBPath:  r.DBPath,
			User:    r.User,
			Decoded: p.decoders.DecodeAll(r.URL),
		}
		v.Flags = p.flagger.FlagVisit(&v)
		visits[i] = v
	}
	return visits
}

// ProcessIncognito converts raw incognito indicators into enriched indicators with decoded URLs.
func (p *Pipeline) ProcessIncognito(raw []model.RawIncognitoIndicator) []model.IncognitoIndicator {
	indicators := make([]model.IncognitoIndicator, len(raw))
	for i, r := range raw {
		indicators[i] = model.IncognitoIndicator{
			URL:     r.URL,
			Browser: r.Browser,
			DBPath:  r.DBPath,
			User:    r.User,
			Decoded: p.decoders.DecodeAll(r.URL),
		}
	}
	return indicators
}
