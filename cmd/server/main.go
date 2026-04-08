package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/error0x001/rutracker/internal/bbcode"
	"github.com/error0x001/rutracker/internal/config"
	"github.com/error0x001/rutracker/internal/db"
	"github.com/error0x001/rutracker/internal/model"
	"github.com/error0x001/rutracker/internal/search"
)

//go:embed web/templates/*.html
var templatesFS embed.FS

//go:embed web/static
var staticFS embed.FS

func staticSubFS() fs.FS {
	f, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		panic(err)
	}
	return f
}

type PageData struct {
	// Index page
	TotalCount int
	Recent     []model.SearchResult
	Categories []string

	// Search page
	Query      string
	Results    []model.SearchResult
	Limit      int
	Offset     int
	Sort       string
	HasMore    bool
	Cat        string
	Subcat     string
	SubSubcat  string
	Subcats    []string
	SubSubcats []string

	// Torrent page
	TorrentTitle string
	ForumName    string
	Category     string
	Size         int64
	RegisteredAt string
	Hash         string
	MagnetLink   template.URL
	Content      template.HTML
	HasFiles     bool
	FileCount    int
	Files        []model.FileEntry

	// Shared
	TotalTorrents int
}

type Server struct {
	searchSvc *search.Service
	templates *template.Template
}

func NewServer(searchSvc *search.Service) (*Server, error) {
	funcMap := template.FuncMap{
		"formatSize": formatSize,
		"shortHash":  shortHash,
		"add":        func(a, b int) int { return a + b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"sub": func(a, b int) int { return a - b },
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templatesFS, "web/templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Server{
		searchSvc: searchSvc,
		templates: tmpl,
	}, nil
}

func (s *Server) RegisterMux(mux *http.ServeMux) {
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/torrent/", s.handleTorrent)
	mux.HandleFunc("/api/search", s.handleAPISearch)
	mux.HandleFunc("/api/torrent/", s.handleAPITorrent)
	mux.HandleFunc("/api/categories", s.handleAPICategories)
	mux.HandleFunc("/api/subcategories", s.handleAPISubcategories)
	mux.HandleFunc("/api/subsubcategories", s.handleAPISubSubcategories)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSubFS()))))
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()
	cats, _ := s.searchSvc.GetCategories(ctx)

	resp, err := s.searchSvc.Search(ctx, model.SearchParams{Limit: 10})
	if err != nil {
		http.Error(w, "Search error", http.StatusInternalServerError)
		log.Printf("search error: %v", err)
		return
	}

	data := PageData{
		TotalCount: resp.Total,
		Recent:     resp.Results,
		Categories: cats,
	}
	s.renderTemplate(w, "page-index", data)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	sortBy := r.URL.Query().Get("sort")
	pageStr := r.URL.Query().Get("p")
	category := r.URL.Query().Get("cat")
	subcategory := r.URL.Query().Get("sub")
	subsubcat := r.URL.Query().Get("sub2")

	limit := 20
	offset := 0
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
			offset = p
		}
	}

	ctx := r.Context()
	resp, err := s.searchSvc.Search(ctx, model.SearchParams{
		Query:       query,
		Limit:       limit,
		Offset:      offset,
		SortBy:      sortBy,
		Category:    category,
		Subcategory: subcategory,
		SubSubcat:   subsubcat,
	})
	if err != nil {
		http.Error(w, "Search error", http.StatusInternalServerError)
		log.Printf("search error: %v", err)
		return
	}

	cats, _ := s.searchSvc.GetCategories(ctx)
	var subs []string
	var subsubcats []string
	if category != "" {
		subs, _ = s.searchSvc.GetSubcategories(ctx, category)
	}
	if category != "" && subcategory != "" {
		subsubcats, _ = s.searchSvc.GetSubSubcategories(ctx, category, subcategory)
	}

	data := PageData{
		Query:      query,
		Results:    resp.Results,
		Limit:      resp.Limit,
		Offset:     resp.Offset,
		Sort:       sortBy,
		HasMore:    resp.HasMore,
		Cat:        category,
		Subcat:     subcategory,
		SubSubcat:  subsubcat,
		Categories: cats,
		Subcats:    subs,
		SubSubcats: subsubcats,
	}
	s.renderTemplate(w, "page-search", data)
}

func (s *Server) handleTorrent(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/torrent/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid torrent ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	torrent, err := s.searchSvc.GetTorrent(ctx, id)
	if err != nil {
		http.Error(w, "Torrent not found", http.StatusNotFound)
		return
	}

	regAt := ""
	if !torrent.RegisteredAt.IsZero() {
		regAt = torrent.RegisteredAt.Format("2006-01-02 15:04:05")
	}

	data := PageData{
		TorrentTitle: torrent.Title,
		ForumName:    torrent.ForumName,
		Category:     torrent.Category,
		Size:         torrent.Size,
		RegisteredAt: regAt,
		Hash:         torrent.Hash,
		MagnetLink:   generateMagnetLink(torrent),
		Content:      template.HTML(bbcode.Render(torrent.Content)),
		HasFiles:     len(torrent.FileList) > 0,
		FileCount:    len(torrent.FileList),
		Files:        torrent.FileList,
	}
	s.renderTemplate(w, "page-torrent", data)
}

func (s *Server) handleAPISearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	sortBy := r.URL.Query().Get("sort")
	category := r.URL.Query().Get("cat")

	ctx := r.Context()
	resp, err := s.searchSvc.Search(ctx, model.SearchParams{
		Query:    query,
		Limit:    limit,
		Offset:   offset,
		SortBy:   sortBy,
		Category: category,
	})
	if err != nil {
		http.Error(w, "Search error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAPITorrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/torrent/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid torrent ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	torrent, err := s.searchSvc.GetTorrent(ctx, id)
	if err != nil {
		http.Error(w, "Torrent not found", http.StatusNotFound)
		return
	}

	torrent.Content = bbcode.Render(torrent.Content)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(torrent)
}

func (s *Server) handleAPICategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cats, err := s.searchSvc.GetCategories(ctx)
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cats)
}

func (s *Server) handleAPISubcategories(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("cat")
	if cat == "" {
		http.Error(w, "Missing cat parameter", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	subs, err := s.searchSvc.GetSubcategories(ctx, cat)
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subs)
}

func (s *Server) handleAPISubSubcategories(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("cat")
	sub := r.URL.Query().Get("sub")
	if cat == "" || sub == "" {
		http.Error(w, "Missing cat or sub parameter", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	subs, err := s.searchSvc.GetSubSubcategories(ctx, cat, sub)
	if err != nil {
		http.Error(w, "Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subs)
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data PageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func formatSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	switch {
	case size >= TB:
		return fmt.Sprintf("%.2f TB", float64(size)/float64(TB))
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func shortHash(hash string) string {
	if len(hash) > 12 {
		return hash[:8] + "..."
	}
	return hash
}

func generateMagnetLink(t model.Torrent) template.URL {
	if t.Hash == "" {
		return ""
	}
	trackerURL := ""
	if t.TrackerID > 0 {
		if t.TrackerID == 1 {
			trackerURL = "http://bt.t-ru.org/ann?magnet"
		} else {
			trackerURL = fmt.Sprintf("http://bt%d.t-ru.org/ann?magnet", t.TrackerID)
		}
	}
	magnet := fmt.Sprintf("magnet:?xt=urn:btih:%s&dn=%s", t.Hash, t.Title)
	if trackerURL != "" {
		magnet += "&tr=" + trackerURL
	}
	return template.URL(magnet)
}

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DB.DSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	fmt.Println("Connected to database:", cfg.DB.Host)

	searchSvc := search.NewService(pool)

	server, err := NewServer(searchSvc)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	mux := http.NewServeMux()
	server.RegisterMux(mux)

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("Server starting on %s\n", cfg.Server.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
