package decoder

import (
	"net/url"
	"strings"

	"github.com/browser-forensics/browser-forensics/internal/model"
)

type YouTube struct{}

func (y *YouTube) Name() string { return "youtube" }

func (y *YouTube) Match(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "youtu.be" {
		return true
	}
	return strings.Contains(host, "youtube.com") && u.Path == "/watch" && u.Query().Has("v")
}

func (y *YouTube) Decode(rawURL string) (model.DecodedURL, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return model.DecodedURL{}, false
	}
	host := strings.ToLower(u.Hostname())

	var videoID string
	if host == "youtu.be" {
		videoID = strings.TrimPrefix(u.Path, "/")
	} else {
		videoID = u.Query().Get("v")
	}
	if videoID == "" {
		return model.DecodedURL{}, false
	}

	return model.DecodedURL{
		Decoder: y.Name(),
		Kind:    "video",
		Data:    map[string]string{"video_id": videoID},
	}, true
}
