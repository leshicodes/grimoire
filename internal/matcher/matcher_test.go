package matcher

import (
	"testing"

	"github.com/leshicodes/grimoire/internal/parser"
	"github.com/leshicodes/grimoire/internal/tcgplayer"
)

func TestMatch(t *testing.T) {
	entries := []parser.CardEntry{
		{Qty: 1, Name: "Sol Ring"},
		{Qty: 1, Name: "Lightning Bolt"},
		{Qty: 1, Name: "Mana Crypt"},
	}

	listings := []tcgplayer.SellerListing{
		{CardName: "Sol Ring", Condition: "Near Mint", Qty: 2, PriceUSD: 2.99},
		{CardName: "sol ring", Condition: "Lightly Played", Qty: 1, PriceUSD: 2.50}, // case variation
		{CardName: "Lightning Bolt", Condition: "Near Mint", Qty: 5, PriceUSD: 0.99},
	}

	results := Match(entries, listings)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	solRing := results[0]
	if !solRing.Found {
		t.Error("Sol Ring should be found")
	}
	if len(solRing.Listings) != 2 {
		t.Errorf("Sol Ring: expected 2 listings, got %d", len(solRing.Listings))
	}

	bolt := results[1]
	if !bolt.Found {
		t.Error("Lightning Bolt should be found")
	}

	manaCrypt := results[2]
	if manaCrypt.Found {
		t.Error("Mana Crypt should not be found")
	}
	if len(manaCrypt.Listings) != 0 {
		t.Errorf("Mana Crypt: expected 0 listings, got %d", len(manaCrypt.Listings))
	}
}

func TestMatchEmpty(t *testing.T) {
	if got := Match(nil, nil); len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	entries := []parser.CardEntry{{Qty: 1, Name: "LIGHTNING BOLT"}}
	listings := []tcgplayer.SellerListing{{CardName: "lightning bolt", Qty: 4}}

	results := Match(entries, listings)
	if !results[0].Found {
		t.Error("match should be case-insensitive")
	}
}
