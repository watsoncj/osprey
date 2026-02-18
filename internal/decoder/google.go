package decoder

import (
	"net/url"
	"strings"

	"github.com/browser-forensics/browser-forensics/internal/model"
)

type GoogleSearch struct{}

func (g *GoogleSearch) Name() string { return "google_search" }

func (g *GoogleSearch) Match(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return (strings.Contains(host, "google.") && u.Path == "/search" && u.Query().Has("q"))
}

func (g *GoogleSearch) Decode(rawURL string) (model.DecodedURL, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return model.DecodedURL{}, false
	}
	q := u.Query().Get("q")
	if q == "" {
		return model.DecodedURL{}, false
	}
	return model.DecodedURL{
		Decoder: g.Name(),
		Kind:    "search",
		Data:    map[string]string{"query": q},
	}, true
}
