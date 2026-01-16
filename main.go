package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

// --------------------------------------------------
// CONFIG & GLOBALS
// --------------------------------------------------
var (
	defaultDictPath  = "barc_csv_file_here"
	globalDictionary []ProductRow
)

// --------------------------------------------------
// API STRUCTS
// --------------------------------------------------

// ExtractionRequest represents the incoming JSON payload
type ExtractionRequest struct {
	RawText string `json:"raw_text"`
}

// ExtractionResponse represents the API response
type ExtractionResponse struct {
	Product   *string `json:"product"`
	Brand     *string `json:"brand"`
	Category  *string `json:"category"`
	Status    string  `json:"status"`
	TimeTaken string  `json:"time_taken"`
}

// --------------------------------------------------
// DOMAIN STRUCTS
// --------------------------------------------------
type ProductRow struct {
	Product     string
	Brand       string
	Category    string
	NormProduct string
	TokenLen    int
}

type Result struct {
	Product  *string
	Brand    *string
	Category *string
	Status   string
}

// --------------------------------------------------
// 1. NORMALIZATION
// --------------------------------------------------
var (
	reTechUnits   = regexp.MustCompile(`(?i)\b\d+(gb|tb|mah|hz|fps|mp)\b`)
	reSpecialChar = regexp.MustCompile(`[^a-z0-9\s]`)
	reWhitespace  = regexp.MustCompile(`\s+`)
	reModelToken  = regexp.MustCompile(`^[a-z]+\d+`)
)

func normalize(text string) string {
	text = html.UnescapeString(text)
	text = strings.ToLower(text)
	text = reTechUnits.ReplaceAllString(text, " ")
	text = reSpecialChar.ReplaceAllString(text, " ")
	text = reWhitespace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// --------------------------------------------------
// 2. TOKEN CLASSIFICATION & MATCHING
// --------------------------------------------------
func isModelToken(token string) bool {
	return reModelToken.MatchString(token)
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	row := make([]int, m+1)
	for i := 0; i <= m; i++ {
		row[i] = i
	}

	for i := 1; i <= n; i++ {
		prev := i
		var val int
		for j := 1; j <= m; j++ {
			if r1[i-1] == r2[j-1] {
				val = row[j-1]
			} else {
				val = min(row[j-1]+1, prev+1, row[j]+1)
			}
			row[j-1] = prev
			prev = val
		}
		row[m] = prev
	}
	return row[m]
}

func fuzzRatio(s1, s2 string) int {
	l1 := len([]rune(s1))
	l2 := len([]rune(s2))
	if l1 == 0 && l2 == 0 {
		return 100
	}
	dist := levenshteinDistance(s1, s2)
	return int(float64(l1+l2-dist) / float64(l1+l2) * 100.0)
}

func tokenMatch(token string, productText string) bool {
	productTokens := strings.Fields(productText)
	if isModelToken(token) {
		for _, pt := range productTokens {
			if token == pt {
				return true
			}
		}
		return false
	}
	for _, pt := range productTokens {
		if fuzzRatio(token, pt) >= 90 {
			return true
		}
	}
	return false
}

// --------------------------------------------------
// 3. DATA LOADING
// --------------------------------------------------
func loadDictionary(path string) []ProductRow {
	var dictRows []ProductRow

	file, err := os.Open(path)
	if err != nil {
		log.Fatal("Error reading CSV file:", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal("Error reading CSV records:", err)
	}

	// Simple header mapping
	headers := map[string]int{}
	if len(records) > 0 {
		for i, h := range records[0] {
			headers[strings.ToLower(h)] = i
		}
	}

	for i := 1; i < len(records); i++ {
		row := records[i]
		pIdx, bIdx, cIdx := 0, 1, 2
		if idx, ok := headers["product"]; ok {
			pIdx = idx
		}
		if idx, ok := headers["brand"]; ok {
			bIdx = idx
		}
		if idx, ok := headers["category"]; ok {
			cIdx = idx
		}

		if len(row) > cIdx {
			p := row[pIdx]
			norm := normalize(p)
			dictRows = append(dictRows, ProductRow{
				Product: p, Brand: row[bIdx], Category: row[cIdx], NormProduct: norm, TokenLen: len(strings.Fields(norm)),
			})
		}
	}
	return dictRows
}

// --------------------------------------------------
// 4. CORE LOGIC
// --------------------------------------------------
func extractProductFromRaw(rawText string, dictionary []ProductRow) Result {
	rawNorm := normalize(rawText)
	rawTokens := strings.Fields(rawNorm)

	candidates := make([]ProductRow, len(dictionary))
	copy(candidates, dictionary)

	// Phase 1: Filter candidates based on raw tokens (Elimination)
	for _, token := range rawTokens {
		var filtered []ProductRow
		for _, row := range candidates {
			if tokenMatch(token, row.NormProduct) {
				filtered = append(filtered, row)
			}
		}

		// If filtering reduced the list but didn't empty it, update candidates
		if len(filtered) > 0 {
			candidates = filtered
		}

		// Optimization: If only 1 left, we are done
		if len(candidates) == 1 {
			break
		}
	}

	// Phase 2: Result Decision
	count := len(candidates)

	if count == 0 {
		return Result{Status: "no_match"}
	}

	// LOGIC CHANGE: Check if candidates < 10 (and > 0)
	if count < 10 {
		type ScoredCandidate struct {
			Row   ProductRow
			Score int
		}

		var scored []ScoredCandidate

		// Calculate Fuzzy Match Score for each candidate against the FULL Raw Text
		for _, cand := range candidates {
			// Using fuzzRatio to compare candidate product vs raw text
			score := fuzzRatio(cand.NormProduct, rawNorm)
			scored = append(scored, ScoredCandidate{Row: cand, Score: score})
		}

		// Sort by Score (Descending)
		sort.Slice(scored, func(i, j int) bool {
			return scored[i].Score > scored[j].Score
		})

		best := scored[0].Row
		return Result{Product: &best.Product, Brand: &best.Brand, Category: &best.Category, Status: "matched_fuzzy_max"}
	}

	// If count >= 10
	return Result{Status: "unmatched_too_many_candidates"}
}

// --------------------------------------------------
// 5. HTTP HANDLERS
// --------------------------------------------------

func extractHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ExtractionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	start := time.Now()

	// Core Logic
	result := extractProductFromRaw(req.RawText, globalDictionary)

	elapsed := time.Since(start)

	resp := ExtractionResponse{
		Product:   result.Product,
		Brand:     result.Brand,
		Category:  result.Category,
		Status:    result.Status,
		TimeTaken: fmt.Sprintf("%.4f ms", float64(elapsed.Microseconds())/1000.0),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// --------------------------------------------------
// 6. MAIN SERVER
// --------------------------------------------------
func main() {
	// 1. Determine Config
	csvPath := os.Getenv("DICT_PATH")
	if csvPath == "" {
		csvPath = defaultDictPath
	}

	// 2. Load Data (ONCE at startup)
	fmt.Printf("Loading dictionary from: %s\n", csvPath)
	globalDictionary = loadDictionary(csvPath)
	fmt.Printf("Dictionary loaded with %d items.\n", len(globalDictionary))

	// 3. Define Routes
	http.HandleFunc("/extract", extractHandler)
	http.HandleFunc("/health", healthHandler)

	// 4. Start Server
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	fmt.Printf("Server starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
