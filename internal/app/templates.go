package app

import "embed"

// templateFS contains the HTML templates bundled with the binary.
//
//go:embed templates/*
var templateFS embed.FS
