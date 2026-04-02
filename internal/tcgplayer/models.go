package tcgplayer

// SellerListing is a single card listing in a seller's inventory.
type SellerListing struct {
	CardName  string
	SetName   string
	Condition string
	Language  string
	Qty       int
	PriceUSD  float64
	TCGID     string // TCGPlayer product ID; used to construct listing URLs
}

// URL returns the TCGPlayer product URL, or empty string when TCGID is unknown.
func (l SellerListing) URL() string {
	if l.TCGID == "" {
		return ""
	}
	return "https://www.tcgplayer.com/product/" + l.TCGID
}
