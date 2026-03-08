package decoder

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/watsoncj/osprey/internal/model"
)

var ytHTTPClient = &http.Client{Timeout: 5 * time.Second}
var ytTitleCache = newYTCache()

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

	data := map[string]string{"video_id": videoID}
	if title, ok := ytTitleCache.get(videoID); ok {
		if title != "" {
			data["title"] = title
		}
	} else if title := fetchYouTubeTitle(videoID); title != "" {
		data["title"] = title
		ytTitleCache.set(videoID, title)
	} else {
		ytTitleCache.set(videoID, "")
	}

	return model.DecodedURL{
		Decoder: y.Name(),
		Kind:    "video",
		Data:    data,
	}, true
}

func fetchYouTubeTitle(videoID string) string {
	oembedURL := fmt.Sprintf("https://www.youtube.com/oembed?url=%s&format=json",
		url.QueryEscape("https://www.youtube.com/watch?v="+videoID))
	resp, err := ytHTTPClient.Get(oembedURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()
	var result struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	return result.Title
}
