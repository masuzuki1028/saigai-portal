package jma

import (
	"encoding/xml"
	"fmt"
	"io"
	"time"
)

// フィードの URL（高頻度フィード。毎分更新）。
const (
	FeedExtra = "https://www.data.jma.go.jp/developer/xml/feed/extra.xml" // 随時（警報・注意報など）
	FeedEqVol = "https://www.data.jma.go.jp/developer/xml/feed/eqvol.xml" // 地震火山
)

// FeedEntry は Atom フィードの1件。
type FeedEntry struct {
	Title   string
	ID      string // 電文の URL
	URL     string
	Updated time.Time
	Author  string
	Content string
}

// ParseFeed は Atom フィードを解析する。
func ParseFeed(r io.Reader) ([]FeedEntry, error) {
	var f struct {
		Entries []struct {
			Title   string `xml:"title"`
			ID      string `xml:"id"`
			Updated string `xml:"updated"`
			Author  string `xml:"author>name"`
			Link    struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
			Content string `xml:"content"`
		} `xml:"entry"`
	}
	if err := xml.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("decode feed: %w", err)
	}
	out := make([]FeedEntry, 0, len(f.Entries))
	for _, e := range f.Entries {
		u, err := time.Parse(time.RFC3339, e.Updated)
		if err != nil {
			return nil, fmt.Errorf("parse updated %q: %w", e.Updated, err)
		}
		out = append(out, FeedEntry{Title: e.Title, ID: e.ID, URL: e.Link.Href, Updated: u, Author: e.Author, Content: e.Content})
	}
	return out, nil
}

// Wanted は取り込む対象の電文か。
func Wanted(title string) bool { return title == TitleWarning || title == TitleQuake }
