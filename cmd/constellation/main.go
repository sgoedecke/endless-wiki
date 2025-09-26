package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"endlesswiki/internal/app"
)

type node struct {
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	Outbound  int       `json:"outbound"`
}

type edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type graph struct {
	GeneratedAt time.Time `json:"generated_at"`
	Nodes       []node    `json:"nodes"`
	Edges       []edge    `json:"edges"`
}

func main() {
	outPath := flag.String("out", "static/constellation.json", "path to write constellation JSON")
	flag.Parse()

	cfg, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := app.NewDB(cfg)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT slug, content, created_at FROM pages`)
	if err != nil {
		log.Fatalf("query pages: %v", err)
	}
	defer rows.Close()

	type page struct {
		slug    string
		content string
		created time.Time
	}

	pages := make([]page, 0)
	slugSet := make(map[string]struct{})

	for rows.Next() {
		var p page
		if err := rows.Scan(&p.slug, &p.content, &p.created); err != nil {
			log.Fatalf("scan page: %v", err)
		}
		pages = append(pages, p)
		slugSet[p.slug] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("iterate pages: %v", err)
	}

	nodes := make([]node, 0, len(pages))
	edges := make([]edge, 0)

	for _, p := range pages {
		links := app.ExtractLinkedSlugs(p.content)
		filtered := make([]string, 0, len(links))
		for _, target := range links {
			if target == p.slug {
				continue
			}
			if _, ok := slugSet[target]; !ok {
				continue
			}
			filtered = append(filtered, target)
			edges = append(edges, edge{Source: p.slug, Target: target})
		}

		nodes = append(nodes, node{
			Slug:      p.slug,
			CreatedAt: p.created,
			Outbound:  len(filtered),
		})
	}

	g := graph{
		GeneratedAt: time.Now().UTC(),
		Nodes:       nodes,
		Edges:       edges,
	}

	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		log.Fatalf("marshal graph: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		log.Fatalf("write graph: %v", err)
	}

	log.Printf("wrote constellation to %s (%d nodes, %d edges)", *outPath, len(nodes), len(edges))
}
