package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/leshicodes/grimoire/internal/importer"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: diag <deck-url>")
		os.Exit(1)
	}
	deckURL := os.Args[1]

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	entries, name, err := importer.FromURL(ctx, deckURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	fmt.Printf("Deck: %s, Cards: %d\n", name, len(entries))
	for i, e := range entries {
		if i < 5 {
			fmt.Printf("  %dx %s\n", e.Qty, e.Name)
		}
	}
}
