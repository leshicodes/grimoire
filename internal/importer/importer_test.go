package importer

import (
	"strings"
	"testing"

	"github.com/leshicodes/scrybrary/pkg/models"
)

func TestMoxfieldIDExtraction(t *testing.T) {
	tests := []struct {
		url     string
		want    string
		wantErr bool
	}{
		{"https://www.moxfield.com/decks/abc123XYZ", "abc123XYZ", false},
		{"https://moxfield.com/decks/abc123XYZ/my-deck", "abc123XYZ", false},
		{"https://moxfield.com/u/user", "", true},
		{"https://archidekt.com/decks/12345", "", true},
	}
	for _, tt := range tests {
		m := moxfieldIDRe.FindStringSubmatch(tt.url)
		if tt.wantErr {
			if m != nil {
				t.Errorf("URL %q: expected no match, got %q", tt.url, m[1])
			}
			continue
		}
		if m == nil {
			t.Errorf("URL %q: expected match %q, got nil", tt.url, tt.want)
			continue
		}
		if m[1] != tt.want {
			t.Errorf("URL %q: got %q, want %q", tt.url, m[1], tt.want)
		}
	}
}

func TestArchidektIDExtraction(t *testing.T) {
	tests := []struct {
		url     string
		want    string
		wantErr bool
	}{
		{"https://archidekt.com/decks/12345678", "12345678", false},
		{"https://archidekt.com/decks/12345678/my-deck", "12345678", false},
		{"https://archidekt.com/u/user", "", true},
		{"https://moxfield.com/decks/abc", "", true},
	}
	for _, tt := range tests {
		m := archidektIDRe.FindStringSubmatch(tt.url)
		if tt.wantErr {
			if m != nil {
				t.Errorf("URL %q: expected no match, got %q", tt.url, m[1])
			}
			continue
		}
		if m == nil {
			t.Errorf("URL %q: expected match %q, got nil", tt.url, tt.want)
			continue
		}
		if m[1] != tt.want {
			t.Errorf("URL %q: got %q, want %q", tt.url, m[1], tt.want)
		}
	}
}

func TestDeckToEntries(t *testing.T) {
	deck := &models.Deck{
		Metadata: models.Metadata{Name: "Test Deck"},
		Cards: []models.Card{
			{Quantity: 1, Name: "Atraxa, Praetors' Voice", Category: "Commander"},
			{Quantity: 1, Name: "Sol Ring", Category: "Artifacts"},
			{Quantity: 4, Name: "Forest", Category: "Lands"},
			// Maybeboard — should be excluded
			{Quantity: 1, Name: "Mana Crypt", Category: "Maybeboard"},
		},
	}

	entries := deckToEntries(deck)

	byName := make(map[string]int, len(entries))
	for _, e := range entries {
		byName[e.Name] = e.Qty
	}

	if byName["Atraxa, Praetors' Voice"] != 1 {
		t.Errorf("Atraxa: got %d, want 1", byName["Atraxa, Praetors' Voice"])
	}
	if byName["Sol Ring"] != 1 {
		t.Errorf("Sol Ring: got %d, want 1", byName["Sol Ring"])
	}
	if byName["Forest"] != 4 {
		t.Errorf("Forest: got %d, want 4", byName["Forest"])
	}
	if byName["Mana Crypt"] != 0 {
		t.Errorf("Mana Crypt should be excluded (Maybeboard), got qty %d", byName["Mana Crypt"])
	}
	// Verify maybeboard entry is not present at all
	for _, e := range entries {
		if strings.EqualFold(e.Name, "Mana Crypt") {
			t.Errorf("Mana Crypt should not appear in entries at all")
		}
	}
}


