package decoder

import (
	"net/url"
	"strings"

	"github.com/watsoncj/osprey/internal/model"
)

type BingSearch struct{}

func (b *BingSearch) Name() string { return "bing_search" }

func (b *BingSearch) Match(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return strings.Contains(host, "bing.com") && strings.HasSuffix(u.Path, "/search") && u.Query().Has("q")
}

func (b *BingSearch) Decode(rawURL string) (model.DecodedURL, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return model.DecodedURL{}, false
	}
	q := u.Query().Get("q")
	if q == "" {
		return model.DecodedURL{}, false
	}
	return model.DecodedURL{
		Decoder: b.Name(),
		Kind:    "search",
		Data:    map[string]string{"query": q},
	}, true
}
