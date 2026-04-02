package tcgplayer

import (
	"encoding/json"
	"testing"
)

func TestResolveSeller(t *testing.T) {
	client := NewHTTPClient()

	tests := []struct {
		name    string
		url     string
		want    string
		wantErr bool
	}{
		{
			name: "standard seller URL",
			url:  "https://www.tcgplayer.com/sellers/Battle-Trading-Cards/fc861742",
			want: "fc861742",
		},
		{
			name: "trailing slash",
			url:  "https://www.tcgplayer.com/sellers/Battle-Trading-Cards/fc861742/",
			want: "fc861742",
		},
		{
			name:    "missing seller key",
			url:     "https://www.tcgplayer.com/sellers/",
			wantErr: true,
		},
		{
			name:    "wrong host path",
			url:     "https://www.tcgplayer.com/search/magic",
			wantErr: true,
		},
		{
			name:    "invalid URL",
			url:     "://not-a-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.resolveSeller(nil, tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected an error, got key %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// realResponseJSON is an abridged version of the actual TCGPlayer response
// captured during Phase 0 research (searching "Spicy Oatmeal Pizza").
const realResponseJSON = `{
  "errors": [],
  "results": [
    {
      "algorithm": "sales_exp_fields_experiment",
      "totalResults": 1,
      "results": [
        {
          "productId": 679122.0,
          "productName": "Spicy Oatmeal Pizza",
          "setName": "Teenage Mutant Ninja Turtles",
          "marketPrice": 0.2,
          "listings": [
            {
              "listingId": 740242081.0,
              "sellerKey": "fc861742",
              "condition": "Near Mint",
              "language": "English",
              "price": 0.4,
              "quantity": 1.0,
              "productId": 679122.0
            }
          ]
        }
      ]
    }
  ]
}`

func TestParseSearchResponse(t *testing.T) {
	var resp searchResponse
	if err := json.Unmarshal([]byte(realResponseJSON), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 outer result, got %d", len(resp.Results))
	}
	outer := resp.Results[0]

	if outer.TotalResults != 1 {
		t.Errorf("totalResults: got %d, want 1", outer.TotalResults)
	}
	if len(outer.Results) != 1 {
		t.Fatalf("expected 1 product result, got %d", len(outer.Results))
	}

	product := outer.Results[0]
	if product.ProductName != "Spicy Oatmeal Pizza" {
		t.Errorf("productName: got %q", product.ProductName)
	}
	if product.SetName != "Teenage Mutant Ninja Turtles" {
		t.Errorf("setName: got %q", product.SetName)
	}
	if product.ProductID != 679122.0 {
		t.Errorf("productId: got %v", product.ProductID)
	}

	if len(product.Listings) != 1 {
		t.Fatalf("expected 1 listing, got %d", len(product.Listings))
	}
	l := product.Listings[0]
	if l.Condition != "Near Mint" {
		t.Errorf("condition: got %q", l.Condition)
	}
	if l.Price != 0.4 {
		t.Errorf("price: got %v, want 0.4", l.Price)
	}
	if l.Quantity != 1.0 {
		t.Errorf("quantity: got %v, want 1.0", l.Quantity)
	}
	if l.SellerKey != "fc861742" {
		t.Errorf("sellerKey: got %q", l.SellerKey)
	}
}

