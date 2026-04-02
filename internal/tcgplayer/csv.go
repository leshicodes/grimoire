package tcgplayer

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseInventoryCSV parses a TCGPlayer seller inventory CSV export into listings.
//
// Expected columns (standard TCGPlayer seller export format):
//
//	TCGplayer Id, Product Line, Set Name, Product Name, Title,
//	Number, Rarity, Condition, TCG Market Price, TCG Direct Low,
//	TCG Low Price With Shipping, TCG Low Price, Total Quantity,
//	Add to Quantity, TCG Marketplace Price
//
// Only "Product Name" (card name), "Set Name", "Condition",
// "Total Quantity", "TCGplayer Id", and price columns are used.
func ParseInventoryCSV(r io.Reader) ([]SellerListing, error) {
	csvReader := csv.NewReader(r)
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV contains no data rows")
	}

	idx := buildColumnIndex(records[0])

	nameCol, ok := columnIndex(idx, "Product Name", "Title", "Name")
	if !ok {
		return nil, fmt.Errorf("CSV is missing a card name column (expected 'Product Name' or 'Title')")
	}

	setCol, hasSet := columnIndex(idx, "Set Name", "Set")
	condCol, hasCond := columnIndex(idx, "Condition")
	qtyCol, hasQty := columnIndex(idx, "Total Quantity", "Quantity", "Qty")
	tcgIDCol, hasTCGID := columnIndex(idx, "TCGplayer Id", "TCGplayer ID", "Product Id")
	priceCol, hasPrice := columnIndex(idx, "TCG Marketplace Price", "TCG Market Price", "Price")

	listings := make([]SellerListing, 0, len(records)-1)
	for _, record := range records[1:] {
		if len(record) <= nameCol {
			continue
		}
		name := strings.TrimSpace(record[nameCol])
		if name == "" {
			continue
		}

		l := SellerListing{CardName: name}

		if hasSet && setCol < len(record) {
			l.SetName = strings.TrimSpace(record[setCol])
		}
		if hasCond && condCol < len(record) {
			l.Condition = strings.TrimSpace(record[condCol])
		}
		if hasQty && qtyCol < len(record) {
			if q, err := strconv.Atoi(strings.TrimSpace(record[qtyCol])); err == nil {
				l.Qty = q
			}
		}
		// Default to 1 if quantity is absent or zero (assume it's actively listed)
		if l.Qty <= 0 {
			l.Qty = 1
		}
		if hasTCGID && tcgIDCol < len(record) {
			l.TCGID = strings.TrimSpace(record[tcgIDCol])
		}
		if hasPrice && priceCol < len(record) {
			priceStr := strings.TrimSpace(record[priceCol])
			priceStr = strings.TrimPrefix(priceStr, "$")
			if p, err := strconv.ParseFloat(priceStr, 64); err == nil {
				l.PriceUSD = p
			}
		}

		listings = append(listings, l)
	}

	return listings, nil
}

func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

// columnIndex returns the index of the first matching column name from candidates.
func columnIndex(idx map[string]int, candidates ...string) (int, bool) {
	for _, name := range candidates {
		if i, ok := idx[name]; ok {
			return i, true
		}
	}
	return 0, false
}
