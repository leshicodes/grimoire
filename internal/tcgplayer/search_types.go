package tcgplayer

// --- Request types ---

type searchRequest struct {
	Algorithm     string         `json:"algorithm"`
	From          int            `json:"from"`
	Size          int            `json:"size"`
	Filters       outerFilters   `json:"filters"`
	ListingSearch listingSearch  `json:"listingSearch"`
	Context       searchContext  `json:"context"`
	Settings      searchSettings `json:"settings"`
	Sort          searchSort     `json:"sort"`
}

type outerFilters struct {
	Term  map[string][]string    `json:"term"`
	Range map[string]interface{} `json:"range"`
	Match map[string]interface{} `json:"match"`
}

type listingSearch struct {
	Context listingSearchContext `json:"context"`
	Filters listingFilters       `json:"filters"`
}

type listingSearchContext struct {
	Cart map[string]interface{} `json:"cart"`
}

type listingFilters struct {
	Term    listingTermFilters  `json:"term"`
	Range   listingRangeFilters `json:"range"`
	Exclude listingExclude      `json:"exclude"`
}

type listingTermFilters struct {
	SellerStatus string   `json:"sellerStatus"`
	ChannelID    int      `json:"channelId"`
	SellerKey    []string `json:"sellerKey"`
	Language     []string `json:"language"`
}

type listingRangeFilters struct {
	Quantity rangeFilter `json:"quantity"`
}

type rangeFilter struct {
	Gte int `json:"gte"`
}

type listingExclude struct {
	ChannelExclusion int `json:"channelExclusion"`
}

type searchContext struct {
	Cart            map[string]interface{} `json:"cart"`
	ShippingCountry string                 `json:"shippingCountry"`
}

type searchSettings struct {
	UseFuzzySearch bool                   `json:"useFuzzySearch"`
	DidYouMean     map[string]interface{} `json:"didYouMean"`
}

type searchSort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// --- Response types ---
//
// Actual response shape (from Phase 0 research):
//
//	{
//	  "errors": [],
//	  "results": [                          ← outerResults (one per search call)
//	    {
//	      "results": [                      ← searchResult (one per product)
//	        {
//	          "productId": 679122.0,
//	          "productName": "Spicy Oatmeal Pizza",
//	          "setName": "Teenage Mutant Ninja Turtles",
//	          "marketPrice": 0.2,
//	          "listings": [                 ← rawListing (one per seller listing)
//	            {
//	              "listingId": 740242081.0,
//	              "sellerKey": "fc861742",
//	              "condition": "Near Mint",
//	              "language": "English",
//	              "price": 0.4,
//	              "quantity": 1.0,
//	              "productId": 679122.0
//	            }
//	          ]
//	        }
//	      ],
//	      "totalResults": 1
//	    }
//	  ]
//	}

type searchResponse struct {
	Results []outerResult `json:"results"`
}

// outerResult is the top-level element in the results array.
// Each search call produces one of these.
type outerResult struct {
	Results      []searchResult `json:"results"`
	TotalResults int            `json:"totalResults"`
}

type searchResult struct {
	ProductID   float64      `json:"productId"`   // TCGPlayer returns numbers as floats
	ProductName string       `json:"productName"`
	SetName     string       `json:"setName"`
	MarketPrice float64      `json:"marketPrice"`
	Listings    []rawListing `json:"listings"`
}

type rawListing struct {
	ListingID float64 `json:"listingId"` // float64 in JSON
	SellerKey string  `json:"sellerKey"`
	Condition string  `json:"condition"`
	Language  string  `json:"language"`
	Price     float64 `json:"price"`
	Quantity  float64 `json:"quantity"` // float64 in JSON
	ProductID float64 `json:"productId"`
}
