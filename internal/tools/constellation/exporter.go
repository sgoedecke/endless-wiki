package constellation

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"endlesswiki/internal/app"
)

type Node struct {
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	Outbound  int       `json:"outbound"`
	Cluster   int       `json:"cluster"`
}

type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type Graph struct {
	GeneratedAt time.Time `json:"generated_at"`
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
}

// Export generates a constellation snapshot written to outPath (if provided)
// and returns the resulting Graph.
func Export(db *sql.DB, outPath string) (Graph, error) {
	rows, err := db.Query(`SELECT slug, content, created_at FROM pages`)
	if err != nil {
		return Graph{}, err
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
			return Graph{}, err
		}
		pages = append(pages, p)
		slugSet[p.slug] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return Graph{}, err
	}

	nodes := make([]Node, 0, len(pages))
	edges := make([]Edge, 0)

	for _, p := range pages {
		links := app.ExtractLinkedSlugs(p.content)
		outbound := 0
		seen := make(map[string]struct{})
		for _, target := range links {
			if target == p.slug {
				continue
			}
			if _, ok := slugSet[target]; !ok {
				continue
			}
			if _, ok := seen[target]; ok {
				continue
			}
			seen[target] = struct{}{}
			outbound++
			edges = append(edges, Edge{Source: p.slug, Target: target})
		}

		nodes = append(nodes, Node{
			Slug:      p.slug,
			CreatedAt: p.created,
			Outbound:  outbound,
		})
	}

	slugs := make([]string, len(nodes))
	for i, node := range nodes {
		slugs[i] = node.Slug
	}

	clusters := computeLouvainClusters(slugs, edges)
	for i, node := range nodes {
		node.Cluster = clusters[node.Slug]
		nodes[i] = node
	}

	g := Graph{
		GeneratedAt: time.Now().UTC(),
		Nodes:       nodes,
		Edges:       edges,
	}

	if outPath != "" {
		if err := write(outPath, g); err != nil {
			return Graph{}, err
		}
	}

	return g, nil
}

func write(outPath string, g Graph) error {
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(outPath, data, 0o644)
}
