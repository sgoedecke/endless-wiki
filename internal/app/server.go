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
)

// Server wires handlers, templates, and external dependencies together.
type Server struct {
	cfg        Config
	db         *sql.DB
	templates  *template.Template
	httpClient *http.Client
	mux        *http.ServeMux
}

// NewServer constructs an HTTP handler ready to serve wiki requests.
func NewServer(db *sql.DB, cfg Config) (*Server, error) {
	tmpl, err := template.New("base").ParseFS(templateFS, "templates/wiki.gohtml", "templates/home.gohtml")
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]any{}
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
		content, err := GeneratePageHTML(ctx, s.httpClient, s.cfg, slug)
		if err != nil {
			log.Printf("generate page %s: %v", slug, err)
			http.Error(w, "failed to generate page", http.StatusInternalServerError)
			return
		}

		page = &Page{Slug: slug, Content: content}
		if err := s.insertPage(ctx, page); err != nil {
			log.Printf("insert page %s: %v", slug, err)
			http.Error(w, "failed to persist page", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		Title   string
		Slug    string
		Content template.HTML
	}{
		Title:   SlugTitle(page.Slug),
		Slug:    page.Slug,
		Content: template.HTML(page.Content),
	}

	if err := s.templates.ExecuteTemplate(w, "wiki.gohtml", data); err != nil {
		log.Printf("render page %s: %v", slug, err)
	}
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

func (s *Server) insertPage(ctx context.Context, page *Page) error {
	const insert = `INSERT INTO pages (slug, content) VALUES (?, ?)`
	_, err := s.db.ExecContext(ctx, insert, page.Slug, page.Content)
	return err
}
