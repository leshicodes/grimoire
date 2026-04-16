package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	qrcode "github.com/skip2/go-qrcode"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/leshicodes/grimoire/internal/importer"
	"github.com/leshicodes/grimoire/internal/matcher"
	"github.com/leshicodes/grimoire/internal/parser"
	"github.com/leshicodes/grimoire/internal/tcgplayer"
)

//go:embed web
var webFS embed.FS

// --- Inventory cache ---

type cachedInventory struct {
	Listings    []tcgplayer.SellerListing
	DisplayName string
	CardCount   int
	UploadedAt  time.Time
}

type storeEntry struct {
	DisplayName string
	StoreName   string
	SellerKey   string
	CardCount   int
	UploadedAt  time.Time
	HasCSV      bool
}

// --- Saved result sessions ---

const sessionTTL = 48 * time.Hour

type savedSession struct {
	Results     resultsData
	DisplayName string
	StoreName   string
	SavedAt     time.Time
}

var (
	invMu    sync.RWMutex
	invStore = map[string]*cachedInventory{}
	storeDir []*storeEntry

	sessionMu sync.RWMutex
	sessions  = map[string]*savedSession{}
)

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// startSessionCleanup launches a goroutine that evicts expired sessions hourly.
func startSessionCleanup() {
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for range t.C {
			now := time.Now()
			sessionMu.Lock()
			for id, s := range sessions {
				if now.Sub(s.SavedAt) > sessionTTL {
					delete(sessions, id)
				}
			}
			sessionMu.Unlock()
		}
	}()
}

func cacheKey(storeName, sellerKey string) string {
	return storeName + "/" + sellerKey
}

// validSlug reports whether s is a safe URL path segment
// (alphanumeric, hyphens, and underscores only).
func validSlug(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// --- Template data types ---

type storeContext struct {
	StoreName   string
	SellerKey   string
	DisplayName string
	CardCount   int
	UploadedAt  time.Time
	LiveMode    bool // true when no CSV cached; live scraping will be used
}

// indexData is the template data passed to the index page.
type indexData struct {
	StoreURL     string
	StoreContext *storeContext
}

// resultsData is the template data passed to the "results" partial.
type resultsData struct {
	Results       []matcher.MatchResult
	Error         string
	Found         int
	NotFound      int
	Total         int
	TotalCost     float64
	CompletionPct int
	FoundNames    string // newline-separated found card names for copy button
	StoreName     string // set when searching a store route (for session save)
	SellerKey     string // set when searching a store route (for session save)
	SessionID     string // ID of the auto-saved share session
}
func main() {
	level := zerolog.InfoLevel
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = zerolog.DebugLevel
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(level)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	tmpl := template.Must(template.ParseFS(webFS, "web/templates/*.html"))

	staticFS, err := fs.Sub(webFS, "web/static")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create static sub-FS")
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		data := indexData{
			StoreURL: r.URL.Query().Get("store"),
		}
		if err := tmpl.ExecuteTemplate(w, "index", data); err != nil {
			log.Error().Err(err).Msg("index template failed")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	})

	// Store-specific route: customers land here via QR code.
	r.Get("/s/{storeName}/{sellerKey}", handleStoreRoute(tmpl))

	r.Get("/setup", func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.ExecuteTemplate(w, "setup", nil); err != nil {
			log.Error().Err(err).Msg("setup template failed")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	})

	r.Get("/stores", func(w http.ResponseWriter, r *http.Request) {
		invMu.RLock()
		stores := make([]*storeEntry, len(storeDir))
		copy(stores, storeDir)
		invMu.RUnlock()
		if err := tmpl.ExecuteTemplate(w, "stores", stores); err != nil {
			log.Error().Err(err).Msg("stores template failed")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	})

	r.Post("/upload", handleUpload(tmpl))
	r.Post("/search", handleSearch(tmpl))
	r.Get("/r/{sessionID}", handleShared(tmpl))
	r.Post("/import", handleImport)
	r.Get("/qr", handleQR)

	startSessionCleanup()

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Info().Str("port", port).Msg("server started — visit http://localhost:" + port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}

// handleStoreRoute renders the index page pre-configured for a specific store.
// Customers arrive here by scanning the store's QR code.
func handleStoreRoute(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		storeName := chi.URLParam(r, "storeName")
		sellerKey := chi.URLParam(r, "sellerKey")
		if !validSlug(storeName) || !validSlug(sellerKey) {
			http.NotFound(w, r)
			return
		}

		sc := &storeContext{
			StoreName:   storeName,
			SellerKey:   sellerKey,
			DisplayName: storeName,
			LiveMode:    true,
		}

		invMu.RLock()
		cached, ok := invStore[cacheKey(storeName, sellerKey)]
		invMu.RUnlock()
		if ok {
			sc.DisplayName = cached.DisplayName
			sc.CardCount = cached.CardCount
			sc.UploadedAt = cached.UploadedAt
			sc.LiveMode = false
		}

		data := indexData{StoreContext: sc}
		if err := tmpl.ExecuteTemplate(w, "index", data); err != nil {
			log.Error().Err(err).Msg("index template failed")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// handleUpload accepts a TCGPlayer seller URL + inventory CSV from a store owner,
// caches the parsed listings, and returns an HTMX partial with the shareable URL and QR code.
func handleUpload(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const maxUpload = 10 << 20 // 10 MB
		if err := r.ParseMultipartForm(maxUpload); err != nil && err != http.ErrNotMultipart {
			http.Error(w, "Failed to parse form.", http.StatusBadRequest)
			return
		}

		sellerURL := strings.TrimSpace(r.FormValue("seller_url"))
		if sellerURL == "" {
			http.Error(w, "seller_url is required", http.StatusBadRequest)
			return
		}

		storeName, sellerKey, err := tcgplayer.ParseSellerURL(sellerURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		csvFile, _, err := r.FormFile("inventory_csv")
		hasCSV := err == nil

		var listings []tcgplayer.SellerListing
		if hasCSV {
			defer csvFile.Close()
			listings, err = tcgplayer.ParseInventoryCSV(csvFile)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to parse CSV: %s", err), http.StatusBadRequest)
				return
			}
		}

		displayName := strings.TrimSpace(r.FormValue("display_name"))
		if displayName == "" {
			displayName = storeName
		}

		inv := &cachedInventory{
			Listings:    listings,
			DisplayName: displayName,
			CardCount:   len(listings),
			UploadedAt:  time.Now(),
		}

		key := cacheKey(storeName, sellerKey)
		invMu.Lock()
		if hasCSV {
			invStore[key] = inv
		}
		// Update or append the store directory entry.
		found := false
		for _, e := range storeDir {
			if e.StoreName == storeName && e.SellerKey == sellerKey {
				e.DisplayName = displayName
				e.CardCount = len(listings)
				e.UploadedAt = inv.UploadedAt
				found = true
				break
			}
		}
		if !found {
			storeDir = append(storeDir, &storeEntry{
				DisplayName: displayName,
				StoreName:   storeName,
				SellerKey:   sellerKey,
				CardCount:   len(listings),
				UploadedAt:  inv.UploadedAt,
				HasCSV:      hasCSV,
			})
		}
		invMu.Unlock()

		log.Info().Str("store", key).Int("cards", len(listings)).Bool("hasCSV", hasCSV).Msg("store registered")

		scheme := "https"
		if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		storeURL := fmt.Sprintf("%s://%s/s/%s/%s", scheme, r.Host, storeName, sellerKey)
		qrURL := fmt.Sprintf("/qr?url=%s", url.QueryEscape(storeURL))

		type uploadResult struct {
			StoreURL    string
			QRCodeURL   string
			DisplayName string
			CardCount   int
			HasCSV      bool
		}
		data := uploadResult{
			StoreURL:    storeURL,
			QRCodeURL:   qrURL,
			DisplayName: displayName,
			CardCount:   len(listings),
			HasCSV:      hasCSV,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "upload-result", data); err != nil {
			log.Error().Err(err).Msg("upload-result template failed")
		}
	}
}

// handleQR generates a QR code PNG for the url query parameter.
func handleQR(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	if rawURL == "" {
		http.Error(w, "url parameter required", http.StatusBadRequest)
		return
	}
	if _, err := url.ParseRequestURI(rawURL); err != nil {
		http.Error(w, "invalid url parameter", http.StatusBadRequest)
		return
	}
	png, err := qrcode.Encode(rawURL, qrcode.Medium, 256)
	if err != nil {
		log.Error().Err(err).Msg("QR code generation failed")
		http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(png)
}

func handleSearch(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const maxUpload = 10 << 20 // 10 MB
		if err := r.ParseMultipartForm(maxUpload); err != nil && err != http.ErrNotMultipart {
			renderResults(w, tmpl, resultsData{Error: "Failed to parse form."})
			return
		}

		cardList := r.FormValue("card_list")
		sellerURL := r.FormValue("seller_url")
		storeNameForm := r.FormValue("store_name")
		sellerKeyForm := r.FormValue("seller_key")

		if cardList == "" {
			renderResults(w, tmpl, resultsData{Error: "Card list cannot be empty."})
			return
		}

		entries := parser.ParseList(cardList)
		if len(entries) == 0 {
			renderResults(w, tmpl, resultsData{Error: "No valid cards found in the provided list."})
			return
		}

		var listings []tcgplayer.SellerListing
		var fetchErr error

		csvFile, _, err := r.FormFile("inventory_csv")
		if err == nil {
			// Explicit CSV upload takes priority.
			defer csvFile.Close()
			listings, fetchErr = tcgplayer.ParseInventoryCSV(csvFile)
		} else if validSlug(storeNameForm) && validSlug(sellerKeyForm) {
			// Store route: check cache first, fall back to live scraping.
			key := cacheKey(storeNameForm, sellerKeyForm)
			invMu.RLock()
			cached, ok := invStore[key]
			invMu.RUnlock()
			if ok {
				listings = cached.Listings
			} else {
				ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
				defer cancel()
				client := tcgplayer.NewHTTPClient()
				names := make([]string, len(entries))
				for i, e := range entries {
					names[i] = e.Name
				}
				builtURL := fmt.Sprintf("https://www.tcgplayer.com/sellers/%s/%s", storeNameForm, sellerKeyForm)
				listings, fetchErr = client.SearchInventory(ctx, builtURL, names)
			}
		} else if sellerURL != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
			defer cancel()
			client := tcgplayer.NewHTTPClient()
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name
			}
			listings, fetchErr = client.SearchInventory(ctx, sellerURL, names)
		} else {
			renderResults(w, tmpl, resultsData{Error: "Please provide a TCGPlayer seller URL or upload an inventory CSV file."})
			return
		}

		if fetchErr != nil {
			log.Warn().Err(fetchErr).Msg("inventory fetch failed")
			renderResults(w, tmpl, resultsData{Error: fetchErr.Error()})
			return
		}

		results := matcher.Match(entries, listings)
		data := resultsData{
			Results:   results,
			Total:     len(results),
			StoreName: storeNameForm,
			SellerKey: sellerKeyForm,
		}

		var foundNames strings.Builder
		for _, res := range results {
			if res.Found {
				data.Found++
				// Use the cheapest listing price for the cost estimate.
				if len(res.Listings) > 0 {
					minPrice := res.Listings[0].PriceUSD
					for _, l := range res.Listings[1:] {
						if l.PriceUSD > 0 && (l.PriceUSD < minPrice || minPrice == 0) {
							minPrice = l.PriceUSD
						}
					}
					data.TotalCost += minPrice
				}
				fmt.Fprintf(&foundNames, "%s\n", res.Entry.Name)
			} else {
				data.NotFound++
			}
		}
		data.FoundNames = strings.TrimRight(foundNames.String(), "\n")
		if data.Total > 0 {
			data.CompletionPct = data.Found * 100 / data.Total
		}

		// Snapshot the results for sharing. Session is auto-saved for all searches.
		if id, err := newSessionID(); err == nil {
			displayName := data.StoreName
			invMu.RLock()
			if cached, ok := invStore[cacheKey(data.StoreName, data.SellerKey)]; ok {
				displayName = cached.DisplayName
			}
			invMu.RUnlock()
			sessionMu.Lock()
			sessions[id] = &savedSession{
				Results:     data,
				DisplayName: displayName,
				StoreName:   data.StoreName,
				SavedAt:     time.Now(),
			}
			sessionMu.Unlock()
			data.SessionID = id
		}

		renderResults(w, tmpl, data)
	}
}

func renderResults(w http.ResponseWriter, tmpl *template.Template, data resultsData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "results", data); err != nil {
		log.Error().Err(err).Msg("results template failed")
	}
}

// handleShared renders the saved read-only results snapshot for a session.
func handleShared(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "sessionID")

		sessionMu.RLock()
		session, ok := sessions[id]
		sessionMu.RUnlock()

		type sharedData struct {
			Expired bool
			Session *savedSession
			Results resultsData
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if !ok {
			if err := tmpl.ExecuteTemplate(w, "shared", sharedData{Expired: true}); err != nil {
				log.Error().Err(err).Msg("shared template failed")
			}
			return
		}

		if err := tmpl.ExecuteTemplate(w, "shared", sharedData{Session: session, Results: session.Results}); err != nil {
			log.Error().Err(err).Msg("shared template failed")
		}
	}
}

// handleImport fetches a deck from Moxfield or Archidekt and returns a
// plaintext card list suitable for populating the card_list textarea.
func handleImport(w http.ResponseWriter, r *http.Request) {
	deckURL := strings.TrimSpace(r.FormValue("deck_url"))
	if deckURL == "" {
		http.Error(w, "deck_url is required", http.StatusBadRequest)
		return
	}

	entries, deckName, err := importer.FromURL(r.Context(), deckURL)
	if err != nil {
		log.Warn().Err(err).Str("url", deckURL).Msg("deck import failed")
		// Return 200 so HTMX swaps the error text into the textarea.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "// Error: %s", err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// Write deck name as a comment header, then one card per line.
	fmt.Fprintf(w, "// %s\n", deckName)
	for _, e := range entries {
		if e.Qty > 1 {
			fmt.Fprintf(w, "%d %s\n", e.Qty, e.Name)
		} else {
			fmt.Fprintf(w, "%s\n", e.Name)
		}
	}
}
