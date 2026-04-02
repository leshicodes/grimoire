package parser

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// CardEntry is a single card entry from a wantlist.
type CardEntry struct {
	Qty  int
	Name string
}

// quantityRe matches an optional leading "N " or "Nx " quantity prefix.
var quantityRe = regexp.MustCompile(`^(\d+)[xX]?\s+(.+)$`)

// ParseList parses a plaintext card list (one card per line) into card entries.
// Duplicate names are merged and quantities are accumulated.
//
// Supported formats:
//
//	Sol Ring
//	1 Sol Ring
//	3x Lightning Bolt
//	// comment line (ignored)
//	# comment line (ignored)
func ParseList(input string) []CardEntry {
	counts := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		entry := parseLine(line)
		if entry.Name != "" {
			counts[entry.Name] += entry.Qty
		}
	}

	entries := make([]CardEntry, 0, len(counts))
	for name, qty := range counts {
		entries = append(entries, CardEntry{Qty: qty, Name: name})
	}
	return entries
}

func parseLine(line string) CardEntry {
	if m := quantityRe.FindStringSubmatch(line); m != nil {
		qty := 1
		fmt.Sscanf(m[1], "%d", &qty)
		return CardEntry{Qty: qty, Name: strings.TrimSpace(m[2])}
	}
	return CardEntry{Qty: 1, Name: line}
}
