package decoder

import (
	"net/url"
	"strings"

	"github.com/watsoncj/osprey/internal/model"
)

type DuckDuckGo struct{}

func (d *DuckDuckGo) Name() string { return "duckduckgo_search" }

func (d *DuckDuckGo) Match(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return strings.Contains(host, "duckduckgo.com") && u.Query().Has("q")
}

func (d *DuckDuckGo) Decode(rawURL string) (model.DecodedURL, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return model.DecodedURL{}, false
	}
	q := u.Query().Get("q")
	if q == "" {
		return model.DecodedURL{}, false
	}
	return model.DecodedURL{
		Decoder: d.Name(),
		Kind:    "search",
		Data:    map[string]string{"query": q},
	}, true
}
