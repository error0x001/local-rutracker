package model

import "time"

type Torrent struct {
	ID           int64
	Title        string
	Hash         string
	TrackerID    int
	Size         int64
	RegisteredAt time.Time
	ForumID      int
	ForumName    string
	Category     string
	Content      string
	FileList     []FileEntry
	OldVersions  []OldVersion
	Dups         []Dup
}

type FileEntry struct {
	Path  string `json:"p"`
	Size  int64  `json:"s"`
	IsDir bool   `json:"d"`
}

type OldVersion struct {
	Hash        string
	Title       string
	VersionTime string
	UnixTS      int64
}

type Dup struct {
	TorrentID  int64
	Confidence int
	DupTitle   string
}

type SearchParams struct {
	Query       string
	Limit       int
	Offset      int
	SortBy      string
	Category    string
	Subcategory string
	SubSubcat   string
}

type SearchResult struct {
	ID           int64
	Title        string
	ForumName    string
	Category     string
	Size         int64
	RegisteredAt time.Time
	Hash         string
}

type SearchResponse struct {
	Results     []SearchResult
	Total       int
	HasMore     bool
	Query       string
	Limit       int
	Offset      int
	Category    string
	Subcategory string
	SubSubcat   string
}
