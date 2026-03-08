package flagging

import (
	"net/url"
	"strings"

	"github.com/watsoncj/osprey/internal/model"
)

// Flagger checks visits against keyword lists organized by category.
type Flagger struct {
	Categories map[string][]string // category -> keywords (lowercase)
}

// DefaultFlagger returns a flagger with built-in keyword lists.
func DefaultFlagger() *Flagger {
	return &Flagger{Categories: defaultCategories()}
}

// FlagVisit checks a visit's URL, title, and decoded data for flagged keywords.
func (f *Flagger) FlagVisit(v *model.Visit) []model.Flag {
	var flags []model.Flag

	urlDecoded, _ := url.PathUnescape(v.URL)
	urlLower := strings.ToLower(urlDecoded)
	titleLower := strings.ToLower(v.Title)

	for category, keywords := range f.Categories {
		for _, kw := range keywords {
			if strings.Contains(urlLower, kw) {
				flags = append(flags, model.Flag{Category: category, Keyword: kw, Source: "url"})
			}
			if titleLower != "" && strings.Contains(titleLower, kw) {
				flags = append(flags, model.Flag{Category: category, Keyword: kw, Source: "title"})
			}
		}
	}

	for _, d := range v.Decoded {
		for _, val := range d.Data {
			valLower := strings.ToLower(val)
			for category, keywords := range f.Categories {
				for _, kw := range keywords {
					if strings.Contains(valLower, kw) {
						flags = append(flags, model.Flag{Category: category, Keyword: kw, Source: "decoded"})
					}
				}
			}
		}
	}

	return dedup(flags)
}

func dedup(flags []model.Flag) []model.Flag {
	seen := make(map[string]bool)
	var out []model.Flag
	for _, f := range flags {
		key := f.Category + "|" + f.Keyword + "|" + f.Source
		if !seen[key] {
			seen[key] = true
			out = append(out, f)
		}
	}
	return out
}
