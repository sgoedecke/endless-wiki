package constellation

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"endlesswiki/internal/app"
)

const maxClusterSample = 40

type ClusterMember struct {
	Slug      string    `json:"slug"`
	Outbound  int       `json:"outbound"`
	CreatedAt time.Time `json:"created_at"`
}

type Cluster struct {
	ID            int             `json:"id"`
	Size          int             `json:"size"`
	Sample        []ClusterMember `json:"sample"`
	InternalLinks int             `json:"internal_links"`
	ExternalLinks int             `json:"external_links"`
	OldestCreated time.Time       `json:"oldest_created_at"`
	NewestCreated time.Time       `json:"newest_created_at"`
}

type ClusterLink struct {
    Source int `json:"source"`
    Target int `json:"target"`
    Weight int `json:"weight"`
}

type Edge struct {
    Source string `json:"source"`
    Target string `json:"target"`
}

type Totals struct {
	Pages    int `json:"pages"`
	Links    int `json:"links"`
	Clusters int `json:"clusters"`
}

type Graph struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Totals      Totals        `json:"totals"`
	Clusters    []Cluster     `json:"clusters"`
	Links       []ClusterLink `json:"links"`
}

type pageRecord struct {
    slug     string
    created  time.Time
    outbound int
}

type clusterStats struct {
	members       []ClusterMember
	internalLinks int
	externalLinks int
	oldest        time.Time
	newest        time.Time
}

type clusterPair struct {
	a int
	b int
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

	pageRecords := make([]pageRecord, 0, len(pages))
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

		pageRecords = append(pageRecords, pageRecord{
			slug:     p.slug,
			created:  p.created,
			outbound: outbound,
		})
	}

	slugs := make([]string, len(pageRecords))
	for i, record := range pageRecords {
		slugs[i] = record.slug
	}

	clusterAssignments := computeLouvainClusters(slugs, edges)

	statsByCluster := make(map[int]*clusterStats)

	for i, record := range pageRecords {
		clusterID, ok := clusterAssignments[record.slug]
		if !ok {
			continue
		}
        stats := statsByCluster[clusterID]
		if stats == nil {
			stats = &clusterStats{}
			statsByCluster[clusterID] = stats
		}
		member := ClusterMember{
			Slug:      record.slug,
			Outbound:  record.outbound,
			CreatedAt: record.created,
		}
		stats.members = append(stats.members, member)
		if !member.CreatedAt.IsZero() {
			if stats.oldest.IsZero() || member.CreatedAt.Before(stats.oldest) {
				stats.oldest = member.CreatedAt
			}
			if stats.newest.IsZero() || member.CreatedAt.After(stats.newest) {
				stats.newest = member.CreatedAt
			}
		}
	}

	linkWeights := make(map[clusterPair]int)

	for _, edge := range edges {
		srcCluster, okSrc := clusterAssignments[edge.Source]
		dstCluster, okDst := clusterAssignments[edge.Target]
		if !okSrc || !okDst {
			continue
		}
		if srcCluster == dstCluster {
			stats := statsByCluster[srcCluster]
			if stats != nil {
				stats.internalLinks++
			}
			continue
		}
		pair := clusterPair{a: srcCluster, b: dstCluster}
		if pair.a > pair.b {
			pair.a, pair.b = pair.b, pair.a
		}
		linkWeights[pair]++

		if stats := statsByCluster[srcCluster]; stats != nil {
			stats.externalLinks++
		}
		if stats := statsByCluster[dstCluster]; stats != nil {
			stats.externalLinks++
		}
	}

	clusterIDs := make([]int, 0, len(statsByCluster))
	for id := range statsByCluster {
		clusterIDs = append(clusterIDs, id)
	}

	sort.Slice(clusterIDs, func(i, j int) bool {
		statsI := statsByCluster[clusterIDs[i]]
		statsJ := statsByCluster[clusterIDs[j]]
		if len(statsI.members) == len(statsJ.members) {
			return clusterIDs[i] < clusterIDs[j]
		}
		return len(statsI.members) > len(statsJ.members)
	})

	clusters := make([]Cluster, 0, len(clusterIDs))

	for _, id := range clusterIDs {
		stats := statsByCluster[id]
		if stats == nil {
			continue
		}
		sort.Slice(stats.members, func(i, j int) bool {
			if stats.members[i].Outbound == stats.members[j].Outbound {
				return stats.members[i].Slug < stats.members[j].Slug
			}
			return stats.members[i].Outbound > stats.members[j].Outbound
		})
		sampleCount := len(stats.members)
		if sampleCount > maxClusterSample {
			sampleCount = maxClusterSample
		}
		sample := make([]ClusterMember, sampleCount)
		copy(sample, stats.members[:sampleCount])

		clusters = append(clusters, Cluster{
			ID:            id,
			Size:          len(stats.members),
			Sample:        sample,
			InternalLinks: stats.internalLinks,
			ExternalLinks: stats.externalLinks,
			OldestCreated: stats.oldest,
			NewestCreated: stats.newest,
		})
	}

	links := make([]ClusterLink, 0, len(linkWeights))
	for pair, weight := range linkWeights {
		links = append(links, ClusterLink{Source: pair.a, Target: pair.b, Weight: weight})
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].Weight == links[j].Weight {
			if links[i].Source == links[j].Source {
				return links[i].Target < links[j].Target
			}
			return links[i].Source < links[j].Source
		}
		return links[i].Weight > links[j].Weight
	})

	g := Graph{
		GeneratedAt: time.Now().UTC(),
		Totals: Totals{
			Pages:    len(pageRecords),
			Links:    len(edges),
			Clusters: len(clusters),
		},
		Clusters: clusters,
		Links:    links,
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
