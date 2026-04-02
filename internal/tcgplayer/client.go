package tcgplayer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Base URLs discovered during Phase 0 research.
const (
	searchBaseURL = "https://mp-search-api.tcgplayer.com/v1/search"

	// pageSize is the number of results per page for inventory searches.
	// 24 matches TCGPlayer's default page size.
	pageSize = 24
)

// InventorySearcher retrieves a seller's inventory listings for a set of card names.
type InventorySearcher interface {
	SearchInventory(ctx context.Context, sellerURL string, cardNames []string) ([]SellerListing, error)
}

// HTTPClient queries TCGPlayer's internal web API.
//
// # ToS Warning
//
// Automated access to TCGPlayer without an approved API key likely violates
// their Terms of Service. Use this software responsibly and at your own risk.
type HTTPClient struct {
	http *http.Client
}

// NewHTTPClient returns a configured HTTPClient.
func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// SearchInventory looks up each card name in a seller's TCGPlayer inventory.
// sellerURL must be a TCGPlayer seller store URL, e.g.:
//
//	https://www.tcgplayer.com/sellers/Battle-Trading-Cards/fc861742
func (c *HTTPClient) SearchInventory(ctx context.Context, sellerURL string, cardNames []string) ([]SellerListing, error) {
	sellerKey, err := c.resolveSeller(ctx, sellerURL)
	if err != nil {
		return nil, err
	}

	var all []SellerListing
	for _, name := range cardNames {
		listings, err := c.searchInventoryByName(ctx, sellerKey, name)
		if err != nil {
			return nil, fmt.Errorf("searching %q: %w", name, err)
		}
		all = append(all, listings...)
	}
	return all, nil
}

// resolveSeller extracts the seller key from a TCGPlayer store URL.
// Expected format: https://www.tcgplayer.com/sellers/{store-name}/{seller-key}
func (c *HTTPClient) resolveSeller(_ context.Context, storeURL string) (string, error) {
	u, err := url.Parse(storeURL)
	if err != nil {
		return "", fmt.Errorf("invalid seller URL %q: %w", storeURL, err)
	}

	// Path: /sellers/Battle-Trading-Cards/fc861742
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) < 2 || segments[0] != "sellers" {
		return "", fmt.Errorf(
			"seller URL must be https://www.tcgplayer.com/sellers/{store-name}/{seller-key}, got: %s",
			storeURL,
		)
	}
	key := segments[len(segments)-1]
	if key == "" {
		return "", fmt.Errorf("could not extract seller key from URL: %s", storeURL)
	}
	return key, nil
}

// searchInventoryByName queries a seller's inventory for a single card name,
// paging through all results and returning every matching listing.
func (c *HTTPClient) searchInventoryByName(ctx context.Context, sellerKey, cardName string) ([]SellerListing, error) {
	var all []SellerListing
	from := 0

	for {
		batch, total, err := c.fetchPage(ctx, sellerKey, cardName, from)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		from += len(batch)
		if from >= total || len(batch) == 0 {
			break
		}
	}

	return all, nil
}

// fetchPage performs one paginated POST to /v1/search/request and returns
// the listings for this page plus the total result count.
func (c *HTTPClient) fetchPage(ctx context.Context, sellerKey, cardName string, from int) ([]SellerListing, int, error) {
	endpoint := fmt.Sprintf("%s/request?q=%s&isList=false", searchBaseURL, url.QueryEscape(cardName))

	reqBody := searchRequest{
		Algorithm: "sales_exp_fields_experiment",
		From:      from,
		Size:      pageSize,
		Filters: outerFilters{
			Term:  map[string][]string{"productLineName": {"magic"}},
			Range: map[string]interface{}{},
			Match: map[string]interface{}{},
		},
		ListingSearch: listingSearch{
			Context: listingSearchContext{Cart: map[string]interface{}{}},
			Filters: listingFilters{
				Term: listingTermFilters{
					SellerStatus: "Live",
					ChannelID:    0,
					SellerKey:    []string{sellerKey},
					Language:     []string{"English"},
				},
				Range:   listingRangeFilters{Quantity: rangeFilter{Gte: 1}},
				Exclude: listingExclude{ChannelExclusion: 0},
			},
		},
		Context: searchContext{
			Cart:            map[string]interface{}{},
			ShippingCountry: "US",
		},
		Settings: searchSettings{
			UseFuzzySearch: true,
			DidYouMean:     map[string]interface{}{},
		},
		Sort: searchSort{Field: "market-price", Order: "desc"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("marshalling search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, err
	}
	setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("TCGPlayer search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("TCGPlayer returned HTTP %d searching %q for seller %q", resp.StatusCode, cardName, sellerKey)
	}

	var searchResp searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, 0, fmt.Errorf("decoding TCGPlayer response: %w", err)
	}

	// Response has two levels of nesting: searchResp.Results[0].Results[n].Listings[m]
	if len(searchResp.Results) == 0 {
		return nil, 0, nil
	}
	outer := searchResp.Results[0]

	var listings []SellerListing
	for _, result := range outer.Results {
		productID := fmt.Sprintf("%.0f", result.ProductID)
		for _, l := range result.Listings {
			listings = append(listings, SellerListing{
				CardName:  result.ProductName,
				SetName:   result.SetName,
				Condition: l.Condition,
				Language:  l.Language,
				Qty:       int(l.Quantity),
				PriceUSD:  l.Price,
				TCGID:     productID,
			})
		}
		// Surface product with no embedded listings using market price.
		if len(result.Listings) == 0 && result.ProductID != 0 {
			listings = append(listings, SellerListing{
				CardName: result.ProductName,
				SetName:  result.SetName,
				Qty:      1,
				PriceUSD: result.MarketPrice,
				TCGID:    productID,
			})
		}
	}

	return listings, outer.TotalResults, nil
}

// setHeaders applies the request headers observed during Phase 0 research.
func setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.7")
	req.Header.Set("Origin", "https://www.tcgplayer.com")
	req.Header.Set("Referer", "https://www.tcgplayer.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
}
