package decoder

import "github.com/watsoncj/osprey/internal/model"

// URLDecoder extracts structured information from URLs.
type URLDecoder interface {
	Name() string
	Match(rawURL string) bool
	Decode(rawURL string) (model.DecodedURL, bool)
}

// Registry holds an ordered list of URL decoders.
type Registry struct {
	decoders []URLDecoder
}

// NewRegistry creates a registry with the given decoders.
func NewRegistry(decoders ...URLDecoder) *Registry {
	return &Registry{decoders: decoders}
}

// DefaultRegistry returns a registry with all built-in decoders.
func DefaultRegistry() *Registry {
	return NewRegistry(
		&GoogleSearch{},
		&BingSearch{},
		&YouTube{},
		&DuckDuckGo{},
	)
}

// DecodeAll runs all matching decoders against the URL.
func (r *Registry) DecodeAll(rawURL string) []model.DecodedURL {
	var out []model.DecodedURL
	for _, d := range r.decoders {
		if d.Match(rawURL) {
			if du, ok := d.Decode(rawURL); ok {
				out = append(out, du)
			}
		}
	}
	return out
}
