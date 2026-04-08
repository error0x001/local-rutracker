package search

import (
	"context"
	"strings"

	"github.com/error0x001/rutracker/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

func (s *Service) Search(ctx context.Context, params model.SearchParams) (model.SearchResponse, error) {
	resp := model.SearchResponse{
		Query:    params.Query,
		Limit:    params.Limit,
		Offset:   params.Offset,
		Category: params.Category,
	}

	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	limit := params.Limit + 1

	queryText := strings.TrimSpace(params.Query)
	categoryFilter := strings.TrimSpace(params.Category)
	subcatFilter := strings.TrimSpace(params.Subcategory)
	subsubcatFilter := strings.TrimSpace(params.SubSubcat)

	var query string
	var args []interface{}
	argIdx := 1

	whereParts := []string{}
	if queryText != "" {
		whereParts = append(whereParts, "search_vector @@ websearch_to_tsquery('russian', $"+string(rune('0'+argIdx))+")")
		args = append(args, queryText)
		argIdx++
	}
	if subcatFilter != "" && subsubcatFilter != "" {
		whereParts = append(whereParts, "forum_name LIKE $"+string(rune('0'+argIdx)))
		args = append(args, categoryFilter+" - "+subcatFilter+" - "+subsubcatFilter+"%")
		argIdx++
	} else if subcatFilter != "" {
		whereParts = append(whereParts, "forum_name LIKE $"+string(rune('0'+argIdx)))
		args = append(args, categoryFilter+" - "+subcatFilter+"%")
		argIdx++
	} else if categoryFilter != "" {
		whereParts = append(whereParts, "category = $"+string(rune('0'+argIdx)))
		args = append(args, categoryFilter)
		argIdx++
	}

	whereClause := ""
	if len(whereParts) > 0 {
		whereClause = " WHERE " + strings.Join(whereParts, " AND ")
	}

	sortClause := "ORDER BY registered_at DESC"
	switch params.SortBy {
	case "date":
		sortClause = "ORDER BY registered_at DESC"
	case "size":
		sortClause = "ORDER BY size DESC"
	}

	query = `
		SELECT id, title, forum_name, category, size, registered_at, hash
		FROM torrents
	` + whereClause + `
	` + sortClause + `
		LIMIT $` + string(rune('0'+argIdx)) + ` OFFSET $` + string(rune('0'+argIdx+1))
	args = append(args, limit, params.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	for rows.Next() {
		if len(resp.Results) >= params.Limit {
			resp.HasMore = true
			break
		}
		var r model.SearchResult
		var forumName, category *string
		if err := rows.Scan(&r.ID, &r.Title, &forumName, &category, &r.Size, &r.RegisteredAt, &r.Hash); err != nil {
			return resp, err
		}
		if forumName != nil {
			r.ForumName = *forumName
		}
		if category != nil {
			r.Category = *category
		}
		resp.Results = append(resp.Results, r)
	}

	if err := rows.Err(); err != nil {
		return resp, err
	}

	// Estimated total from pg_class stats
	var est int64
	if categoryFilter != "" {
		err = s.pool.QueryRow(ctx, `
			SELECT reltuples::bigint FROM pg_class WHERE relname = 'torrents'
		`).Scan(&est)
	} else {
		err = s.pool.QueryRow(ctx, `
			SELECT reltuples::bigint FROM pg_class WHERE relname = 'torrents'
		`).Scan(&est)
	}
	if err == nil && est > 0 {
		resp.Total = int(est)
	}

	return resp, nil
}

func (s *Service) GetTorrent(ctx context.Context, id int64) (model.Torrent, error) {
	var t model.Torrent
	var forumName, category *string

	err := s.pool.QueryRow(ctx, `
		SELECT id, title, hash, tracker_id, size, registered_at, forum_id, forum_name, category, content, file_list
		FROM torrents WHERE id = $1
	`, id).Scan(&t.ID, &t.Title, &t.Hash, &t.TrackerID, &t.Size, &t.RegisteredAt, &t.ForumID, &forumName, &category, &t.Content, &t.FileList)
	if err != nil {
		return t, err
	}
	if forumName != nil {
		t.ForumName = *forumName
	}
	if category != nil {
		t.Category = *category
	}

	// Load old versions
	oldRows, err := s.pool.Query(ctx, `
		SELECT hash, title, version_time, unixts FROM torrent_old_versions WHERE torrent_id = $1
	`, id)
	if err == nil {
		defer oldRows.Close()
		for oldRows.Next() {
			var ov model.OldVersion
			if err := oldRows.Scan(&ov.Hash, &ov.Title, &ov.VersionTime, &ov.UnixTS); err != nil {
				continue
			}
			t.OldVersions = append(t.OldVersions, ov)
		}
	}

	// Load dups
	dupRows, err := s.pool.Query(ctx, `
		SELECT dup_id, confidence, dup_title FROM torrent_dups WHERE torrent_id = $1
	`, id)
	if err == nil {
		defer dupRows.Close()
		for dupRows.Next() {
			var d model.Dup
			if err := dupRows.Scan(&d.TorrentID, &d.Confidence, &d.DupTitle); err != nil {
				continue
			}
			t.Dups = append(t.Dups, d)
		}
	}

	return t, nil
}

func (s *Service) GetCategories(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT category FROM torrents WHERE category IS NOT NULL AND category != '' ORDER BY category
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cats []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			continue
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (s *Service) GetSubcategories(ctx context.Context, category string) ([]string, error) {
	// Extract second-level parts from forum_name where category matches
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT split_part(forum_name, ' - ', 2) as subcat
		FROM torrents
		WHERE category = $1 AND forum_name LIKE '% - %'
		ORDER BY subcat
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []string
	for rows.Next() {
		var sub string
		if err := rows.Scan(&sub); err != nil {
			continue
		}
		if sub != "" {
			subs = append(subs, sub)
		}
	}
	return subs, rows.Err()
}

func (s *Service) GetSubSubcategories(ctx context.Context, category, subcategory string) ([]string, error) {
	prefix := category + " - " + subcategory + " - "
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT split_part(forum_name, ' - ', 3) as subsubcat
		FROM torrents
		WHERE forum_name LIKE $1
		ORDER BY subsubcat
	`, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []string
	for rows.Next() {
		var sub string
		if err := rows.Scan(&sub); err != nil {
			continue
		}
		if sub != "" {
			subs = append(subs, sub)
		}
	}
	return subs, rows.Err()
}
