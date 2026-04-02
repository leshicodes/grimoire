package matcher

import (
	"strings"

	"github.com/leshicodes/grimoire/internal/parser"
	"github.com/leshicodes/grimoire/internal/tcgplayer"
)

// MatchResult describes whether a wanted card was found in the seller's inventory.
type MatchResult struct {
	Entry    parser.CardEntry
	Found    bool
	Listings []tcgplayer.SellerListing
}

// Match cross-references a wantlist against an inventory and returns one
// MatchResult per entry. Matching is case-insensitive.
func Match(entries []parser.CardEntry, listings []tcgplayer.SellerListing) []MatchResult {
	byName := make(map[string][]tcgplayer.SellerListing, len(listings))
	for _, l := range listings {
		key := normalize(l.CardName)
		byName[key] = append(byName[key], l)
	}

	results := make([]MatchResult, len(entries))
	for i, e := range entries {
		found := byName[normalize(e.Name)]
		results[i] = MatchResult{
			Entry:    e,
			Found:    len(found) > 0,
			Listings: found,
		}
	}
	return results
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
