package importer

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	scryarchidekt "github.com/leshicodes/scrybrary/pkg/adapters/archidekt"
	scrymoxfield "github.com/leshicodes/scrybrary/pkg/adapters/moxfield"
	"github.com/leshicodes/scrybrary/pkg/models"

	"github.com/leshicodes/grimoire/internal/parser"
)

var (
	moxfieldIDRe  = regexp.MustCompile(`moxfield\.com/decks/([A-Za-z0-9_-]+)`)
	archidektIDRe = regexp.MustCompile(`archidekt\.com/decks/(\d+)`)
)

// FromURL detects the deck source from the URL and returns the card list.
// Supported: Moxfield, Archidekt.
func FromURL(ctx context.Context, deckURL string) ([]parser.CardEntry, string, error) {
	switch {
	case strings.Contains(deckURL, "moxfield.com"):
		return fromMoxfield(ctx, deckURL)
	case strings.Contains(deckURL, "archidekt.com"):
		return fromArchidekt(ctx, deckURL)
	default:
		return nil, "", fmt.Errorf("unsupported deck URL — supported sources: Moxfield, Archidekt")
	}
}

func fromMoxfield(ctx context.Context, deckURL string) ([]parser.CardEntry, string, error) {
	m := moxfieldIDRe.FindStringSubmatch(deckURL)
	if m == nil {
		return nil, "", fmt.Errorf("could not extract deck ID from Moxfield URL: %s", deckURL)
	}

	deck, err := scrymoxfield.New().Pull(ctx, m[1])
	if err != nil {
		return nil, "", err
	}

	return deckToEntries(deck), deck.Metadata.Name, nil
}

func fromArchidekt(ctx context.Context, deckURL string) ([]parser.CardEntry, string, error) {
	m := archidektIDRe.FindStringSubmatch(deckURL)
	if m == nil {
		return nil, "", fmt.Errorf("could not extract deck ID from Archidekt URL: %s", deckURL)
	}

	deck, err := scryarchidekt.New().Pull(ctx, m[1])
	if err != nil {
		return nil, "", err
	}

	return deckToEntries(deck), deck.Metadata.Name, nil
}

// deckToEntries converts a scrybrary Deck into the flat card list used by the
// rest of Grimoire. Cards in the Maybeboard category are excluded.
func deckToEntries(deck *models.Deck) []parser.CardEntry {
	counts := make(map[string]int)
	for _, c := range deck.Cards {
		if strings.EqualFold(c.Category, "maybeboard") {
			continue
		}
		name := strings.TrimSpace(c.Name)
		if name != "" {
			counts[name] += c.Quantity
		}
	}

	entries := make([]parser.CardEntry, 0, len(counts))
	for name, qty := range counts {
		entries = append(entries, parser.CardEntry{Qty: qty, Name: name})
	}
	return entries
}

