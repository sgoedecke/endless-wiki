package app

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"golang.org/x/sync/singleflight"
)

// Server wires handlers, templates, and external dependencies together.
type Server struct {
	cfg        Config
	db         *sql.DB
	templates  *template.Template
	httpClient *http.Client
	mux        *http.ServeMux
	genGroup   singleflight.Group
}

// ErrDuplicatePage signals that a slug already exists in the database.
var ErrDuplicatePage = errors.New("duplicate page")

// NewServer constructs an HTTP handler ready to serve wiki requests.
func NewServer(db *sql.DB, cfg Config) (*Server, error) {
	tmpl, err := template.New("base").Funcs(template.FuncMap{
		"slugTitle": SlugTitle,
	}).ParseFS(templateFS, "templates/wiki.gohtml", "templates/home.gohtml", "templates/search.gohtml")
	if err != nil {
		return nil, err
	}

	srv := &Server{
		cfg:       cfg,
		db:        db,
		templates: tmpl,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		mux: http.NewServeMux(),
	}

	srv.mux.HandleFunc("/", srv.handleIndex)
	srv.mux.HandleFunc("/wiki/", srv.handleWiki)
	srv.mux.HandleFunc("/random", srv.handleRandomPage)
	srv.mux.HandleFunc("/recent", srv.handleRecentPage)
	srv.mux.HandleFunc("/search", srv.handleSearch)

	return srv, nil
}

// ServeHTTP satisfies http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	count, err := s.pageCount(r.Context())
	if err != nil {
		log.Printf("count pages: %v", err)
		count = 0
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		PageCount   int
		SearchQuery string
	}{
		PageCount:   count,
		SearchQuery: "",
	}
	if err := s.templates.ExecuteTemplate(w, "home.gohtml", data); err != nil {
		log.Printf("render home: %v", err)
	}
}

func (s *Server) handleWiki(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	raw := strings.TrimPrefix(r.URL.Path, "/wiki/")
	if raw == "" {
		http.Redirect(w, r, "/wiki/main_page", http.StatusFound)
		return
	}

	decoded, err := url.PathUnescape(raw)
	if err != nil {
		http.Error(w, "bad slug", http.StatusBadRequest)
		return
	}

	slug, err := NormalizeSlug(decoded)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()
	page, err := s.lookupPage(ctx, slug)
	if err != nil {
		log.Printf("lookup page %s: %v", slug, err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	if page == nil {
		result, genErr, _ := s.genGroup.Do(slug, func() (interface{}, error) {
			return s.generateAndStore(ctx, slug)
		})
		if genErr != nil {
			log.Printf("generate page %s: %v", slug, genErr)
			http.Error(w, "failed to generate page", http.StatusInternalServerError)
			return
		}

		var ok bool
		page, ok = result.(*Page)
		if !ok {
			log.Printf("generation result type mismatch for %s", slug)
			http.Error(w, "failed to generate page", http.StatusInternalServerError)
			return
		}
	}

	count, err := s.pageCount(ctx)
	if err != nil {
		log.Printf("page count: %v", err)
		count = 0
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		Title       string
		Slug        string
		Content     template.HTML
		PageCount   int
		SearchQuery string
	}{
		Title:       SlugTitle(page.Slug),
		Slug:        page.Slug,
		Content:     template.HTML(page.Content),
		PageCount:   count,
		SearchQuery: "",
	}

	if err := s.templates.ExecuteTemplate(w, "wiki.gohtml", data); err != nil {
		log.Printf("render page %s: %v", slug, err)
	}
}

func (s *Server) handleRandomPage(w http.ResponseWriter, r *http.Request) {
	slug, err := s.randomSlug(r.Context())
	if err != nil {
		log.Printf("random slug: %v", err)
		http.Error(w, "failed to load random page", http.StatusInternalServerError)
		return
	}
	if slug == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/wiki/"+url.PathEscape(slug), http.StatusFound)
}

func (s *Server) handleRecentPage(w http.ResponseWriter, r *http.Request) {
	slug, err := s.recentSlug(r.Context())
	if err != nil {
		log.Printf("recent slug: %v", err)
		http.Error(w, "failed to load recent page", http.StatusInternalServerError)
		return
	}
	if slug == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/wiki/"+url.PathEscape(slug), http.StatusFound)
}

func (s *Server) lookupPage(ctx context.Context, slug string) (*Page, error) {
	const query = `SELECT slug, content, created_at FROM pages WHERE slug = ?`
	row := s.db.QueryRowContext(ctx, query, slug)
	var p Page
	if err := row.Scan(&p.Slug, &p.Content, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.FormValue("q"))
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if len(query) > 128 {
		query = query[:128]
	}

	ctx := r.Context()
	results, err := s.searchPages(ctx, query)
	if err != nil {
		log.Printf("search %q: %v", query, err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	count, err := s.pageCount(ctx)
	if err != nil {
		log.Printf("page count: %v", err)
		count = 0
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		Query       string
		Results     []string
		PageCount   int
		SearchQuery string
	}{
		Query:       query,
		Results:     results,
		PageCount:   count,
		SearchQuery: query,
	}

	if err := s.templates.ExecuteTemplate(w, "search.gohtml", data); err != nil {
		log.Printf("render search: %v", err)
	}
}

func (s *Server) insertPage(ctx context.Context, page *Page) error {
	const insert = `INSERT INTO pages (slug, content) VALUES (?, ?)`
	if _, err := s.db.ExecContext(ctx, insert, page.Slug, page.Content); err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrDuplicatePage
		}
		return err
	}
	return nil
}

func (s *Server) generateAndStore(ctx context.Context, slug string) (*Page, error) {
	content, err := GeneratePageHTML(ctx, s.httpClient, s.cfg, slug)
	if err != nil {
		return nil, err
	}

	page := &Page{Slug: slug, Content: content}
	err = s.insertPage(ctx, page)
	if err == nil {
		return page, nil
	}
	if errors.Is(err, ErrDuplicatePage) {
		// Another request persisted the page before us; fetch the stored version.
		stored, lookupErr := s.lookupPage(ctx, slug)
		if lookupErr != nil {
			return nil, lookupErr
		}
		if stored != nil {
			return stored, nil
		}
		return nil, err
	}
	return nil, err
}

func (s *Server) randomSlug(ctx context.Context) (string, error) {
	const query = `SELECT slug FROM pages ORDER BY RAND() LIMIT 1`
	row := s.db.QueryRowContext(ctx, query)
	var slug string
	if err := row.Scan(&slug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return slug, nil
}

func (s *Server) recentSlug(ctx context.Context) (string, error) {
	const query = `SELECT slug FROM pages ORDER BY created_at DESC LIMIT 1`
	row := s.db.QueryRowContext(ctx, query)
	var slug string
	if err := row.Scan(&slug); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return slug, nil
}

func (s *Server) pageCount(ctx context.Context) (int, error) {
	const query = `SELECT COUNT(*) FROM pages`
	row := s.db.QueryRowContext(ctx, query)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Server) searchPages(ctx context.Context, query string) ([]string, error) {
	const sqlQuery = `SELECT slug FROM pages WHERE slug LIKE ? OR content LIKE ? ORDER BY created_at DESC LIMIT 20`
	like := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, sqlQuery, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return slugs, nil
}
