package migrator

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ulikunitz/xz"
)

type TorrentXML struct {
	XMLName     xml.Name        `xml:"torrent"`
	ID          int64           `xml:"id,attr"`
	RegistredAt string          `xml:"registred_at,attr"`
	UnixTS      int64           `xml:"unixts,attr"`
	Size        int64           `xml:"size,attr"`
	Title       string          `xml:"title"`
	Torrent     TorrentInfo     `xml:"torrent"`
	Forum       ForumInfo       `xml:"forum"`
	Content     string          `xml:"content"`
	Dir         DirEntry        `xml:"dir"`
	OldVersions []OldVersionXML `xml:"old"`
	Dups        []DupXML        `xml:"dup"`
}

type TorrentInfo struct {
	Hash      string `xml:"hash,attr"`
	TrackerID int    `xml:"tracker_id,attr"`
}

type ForumInfo struct {
	ID   int    `xml:"id,attr"`
	Name string `xml:",chardata"`
}

type DirEntry struct {
	Name  string     `xml:"name,attr"`
	Files []FileXML  `xml:"file"`
	Dirs  []DirEntry `xml:"dir"`
}

type FileXML struct {
	Name string `xml:"name,attr"`
	Size int64  `xml:"size,attr"`
}

type OldVersionXML struct {
	Hash   string `xml:"hash,attr"`
	Time   string `xml:"time,attr"`
	UnixTS int64  `xml:"unixts,attr"`
	Title  string `xml:",chardata"`
}

type DupXML struct {
	Confidence int    `xml:"p,attr"`
	ID         int64  `xml:"id,attr"`
	Title      string `xml:",chardata"`
}

type Migrator struct {
	pool          *pgxpool.Pool
	progressEvery int
}

func NewMigrator(pool *pgxpool.Pool, progressEvery int) *Migrator {
	return &Migrator{
		pool:          pool,
		progressEvery: progressEvery,
	}
}

type Checkpoint struct {
	StreamPos int64
	Inserted  int64
}

func (m *Migrator) LoadCheckpoint(ctx context.Context) (Checkpoint, error) {
	cp := Checkpoint{}
	var lastID int64
	err := m.pool.QueryRow(ctx,
		`SELECT last_torrent_id, processed_count FROM migration_checkpoint WHERE id = 1`,
	).Scan(&lastID, &cp.StreamPos)
	if err != nil {
		return cp, err
	}

	var actualInserted int64
	_ = m.pool.QueryRow(ctx, `SELECT count(*) FROM torrents`).Scan(&actualInserted)
	cp.Inserted = actualInserted

	if cp.StreamPos < cp.Inserted {
		cp.StreamPos = cp.Inserted
	}

	fmt.Printf("DB has %d torrents, will resume from stream position ~%d\n", cp.Inserted, cp.StreamPos)
	return cp, nil
}

func (m *Migrator) SaveCheckpoint(ctx context.Context, cp Checkpoint) error {
	_, err := m.pool.Exec(ctx,
		`UPDATE migration_checkpoint SET last_torrent_id = $1, processed_count = $2, updated_at = NOW() WHERE id = 1`,
		cp.StreamPos, cp.Inserted,
	)
	return err
}

func extractCategory(forumName string) string {
	if idx := strings.Index(forumName, " - "); idx != -1 {
		return strings.TrimSpace(forumName[:idx])
	}
	return strings.TrimSpace(forumName)
}

func (m *Migrator) MigrateFile(ctx context.Context, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	cp, err := m.LoadCheckpoint(ctx)
	if err != nil {
		return fmt.Errorf("load checkpoint: %w", err)
	}

	xzReader, err := xz.NewReader(f)
	if err != nil {
		return fmt.Errorf("xz decompressor: %w", err)
	}

	decoder := xml.NewDecoder(xzReader)
	startTime := time.Now()

	var streamPos int64
	var batchTorrents []TorrentXML
	batchSize := 500

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if len(batchTorrents) > 0 {
				n, err := m.insertBatch(ctx, batchTorrents)
				if err != nil {
					return fmt.Errorf("insert final batch: %w", err)
				}
				cp.Inserted += int64(n)
			}
			break
		}
		if err != nil {
			return fmt.Errorf("xml token: %w", err)
		}

		startElem, ok := token.(xml.StartElement)
		if !ok || startElem.Name.Local != "torrent" {
			continue
		}

		streamPos++

		if streamPos <= cp.StreamPos {
			if err := skipElement(decoder); err != nil {
				return fmt.Errorf("skip element: %w", err)
			}
			continue
		}

		var t TorrentXML
		if err := decoder.DecodeElement(&t, &startElem); err != nil {
			return fmt.Errorf("decode torrent at pos %d: %w", streamPos, err)
		}

		batchTorrents = append(batchTorrents, t)

		if len(batchTorrents) >= batchSize {
			n, err := m.insertBatch(ctx, batchTorrents)
			if err != nil {
				return fmt.Errorf("insert batch: %w", err)
			}
			cp.Inserted += int64(n)
			batchTorrents = batchTorrents[:0]

			cp.StreamPos = streamPos
			if err := m.SaveCheckpoint(ctx, cp); err != nil {
				fmt.Printf("WARN: checkpoint save failed: %v\n", err)
			}

			if cp.Inserted > 0 && cp.Inserted%int64(m.progressEvery) == 0 {
				elapsed := time.Since(startTime)
				rate := float64(cp.Inserted) / elapsed.Seconds()
				fmt.Printf("  Inserted: %d | Stream pos: %d | %.0f/sec | %v\n",
					cp.Inserted, cp.StreamPos, rate, elapsed.Round(time.Second))
			}
		}
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\nMigration complete!\n")
	fmt.Printf("  Total: %d | Final stream pos: %d\n", cp.Inserted, streamPos)
	fmt.Printf("  Duration: %s | Rate: %.0f/sec\n",
		elapsed.Round(time.Second), float64(cp.Inserted)/elapsed.Seconds())

	return nil
}

func skipElement(decoder *xml.Decoder) error {
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func (m *Migrator) insertBatch(ctx context.Context, torrents []TorrentXML) (int, error) {
	batch := &pgx.Batch{}

	for _, t := range torrents {
		registeredAt := parseTime(t.RegistredAt, t.UnixTS)
		category := extractCategory(t.Forum.Name)

		// Build file list as JSON
		files := flattenFiles(t.Dir, "")
		fileJSON, _ := json.Marshal(fileListToJSON(files))

		batch.Queue(
			`INSERT INTO torrents (id, title, hash, tracker_id, size, registered_at, forum_id, forum_name, category, content, file_list)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT (id) DO NOTHING`,
			t.ID, t.Title, t.Torrent.Hash, t.Torrent.TrackerID, t.Size, registeredAt,
			t.Forum.ID, strings.TrimSpace(t.Forum.Name), category, t.Content, fileJSON,
		)

		for _, ov := range t.OldVersions {
			batch.Queue(
				`INSERT INTO torrent_old_versions (torrent_id, hash, title, version_time, unixts)
				 VALUES ($1,$2,$3,$4,$5) ON CONFLICT (torrent_id, hash, title) DO NOTHING`,
				t.ID, ov.Hash, strings.TrimSpace(ov.Title), ov.Time, ov.UnixTS,
			)
		}

		for _, d := range t.Dups {
			batch.Queue(
				`INSERT INTO torrent_dups (torrent_id, dup_id, confidence, dup_title)
				 VALUES ($1,$2,$3,$4) ON CONFLICT (torrent_id, dup_id) DO NOTHING`,
				t.ID, d.ID, d.Confidence, strings.TrimSpace(d.Title),
			)
		}
	}

	br := m.pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return 0, fmt.Errorf("batch exec %d: %w", i, err)
		}
	}
	return len(torrents), nil
}

type fileEntry struct {
	Path  string
	Size  int64
	IsDir bool
}

func flattenFiles(dir DirEntry, prefix string) []fileEntry {
	var files []fileEntry
	currentPath := prefix
	if dir.Name != "" {
		if currentPath != "" {
			currentPath += "/" + dir.Name
		} else {
			currentPath = dir.Name
		}
		files = append(files, fileEntry{Path: currentPath, Size: 0, IsDir: true})
	}
	for _, f := range dir.Files {
		fp := currentPath
		if fp != "" {
			fp += "/" + f.Name
		} else {
			fp = f.Name
		}
		files = append(files, fileEntry{Path: fp, Size: f.Size, IsDir: false})
	}
	for _, subDir := range dir.Dirs {
		files = append(files, flattenFiles(subDir, currentPath)...)
	}
	return files
}

type fileJSON struct {
	Path  string `json:"p"`
	Size  int64  `json:"s"`
	IsDir bool   `json:"d"`
}

func fileListToJSON(files []fileEntry) []fileJSON {
	out := make([]fileJSON, len(files))
	for i, f := range files {
		out[i] = fileJSON{Path: f.Path, Size: f.Size, IsDir: f.IsDir}
	}
	return out
}

func parseTime(registredAt string, unixTS int64) time.Time {
	if t, err := time.Parse("2006.01.02 15:04:05", registredAt); err == nil {
		return t
	}
	if unixTS > 0 {
		return time.Unix(unixTS, 0)
	}
	return time.Time{}
}
