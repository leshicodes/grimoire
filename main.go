package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/leshicodes/grimoire/internal/importer"
	"github.com/leshicodes/grimoire/internal/matcher"
	"github.com/leshicodes/grimoire/internal/parser"
	"github.com/leshicodes/grimoire/internal/tcgplayer"
)

//go:embed web
var webFS embed.FS

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

	r.Post("/search", handleSearch(tmpl))
	r.Post("/import", handleImport)

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

// indexData is the template data passed to the index page.
type indexData struct {
	StoreURL string
}

// resultsData is the template data passed to the "results" partial.
type resultsData struct {
	Results  []matcher.MatchResult
	Error    string
	Found    int
	NotFound int
	Total    int
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
			defer csvFile.Close()
			listings, fetchErr = tcgplayer.ParseInventoryCSV(csvFile)
		} else if sellerURL != "" {
			client := tcgplayer.NewHTTPClient()
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name
			}
			listings, fetchErr = client.SearchInventory(r.Context(), sellerURL, names)
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
		data := resultsData{Results: results, Total: len(results)}
		for _, res := range results {
			if res.Found {
				data.Found++
			} else {
				data.NotFound++
			}
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
	// Write deck name as a comment header, then one card per line
	fmt.Fprintf(w, "// %s\n", deckName)
	for _, e := range entries {
		if e.Qty > 1 {
			fmt.Fprintf(w, "%d %s\n", e.Qty, e.Name)
		} else {
			fmt.Fprintf(w, "%s\n", e.Name)
		}
	}
}
