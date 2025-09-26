//go:build ignore

package main

import (
	"flag"
	"log"

	"endlesswiki/internal/app"
	"endlesswiki/internal/tools/constellation"
)

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

	g, err := constellation.Export(db, *outPath)
	if err != nil {
		log.Fatalf("export constellation: %v", err)
	}

	log.Printf("wrote constellation to %s (%d clusters, %d pages, %d links)", *outPath, len(g.Clusters), g.Totals.Pages, g.Totals.Links)
}
