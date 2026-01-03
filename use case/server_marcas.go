package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// =====================
// API DTOs
// =====================

type LoadDBResponse struct {
	Ok          bool   `json:"ok"`
	Size        int64  `json:"size"`
	Mode        string `json:"mode"`
	TotalMarcas int    `json:"total_marcas"`
	DecodeMode  string `json:"decode_mode"`
	Error       string `json:"error,omitempty"`
}

type CheckBrandRequest struct {
	Brand string `json:"brand"`
	TopN  int    `json:"top_n"`
}

type CheckBrandResponse struct {
	Mode   string       `json:"mode"`
	Report *CheckResult `json:"report,omitempty"`
	Error  string       `json:"error,omitempty"`
}

type Candidate struct {
	Nome      string  `json:"nome"`
	BytesPct  float64 `json:"bytes_pct"`
	Sha256Pct float64 `json:"sha256_pct"`
	WinnerPct float64 `json:"winner_pct"`
	Sha256Hex string  `json:"sha256_hex"`
}

type CheckResult struct {
	Query       string      `json:"query"`
	TotalMarcas int         `json:"total_marcas"`
	BinPath     string      `json:"bin_path"`
	DecodeMode  string      `json:"decode_mode"`
	Best        *Candidate  `json:"best,omitempty"`
	Top         []Candidate `json:"top,omitempty"`
	Message     string      `json:"message"`
}

// =====================
// State
// =====================

type dbEntry struct {
	Name      string
	BitsText  []byte
	BitsHash  []byte
	Sha256Hex string
}

var dbMu sync.RWMutex
var loadedDBPath string
var loadedEntries []dbEntry
var loadedDecodeMode string

func setLoadedDB(path string, entries []dbEntry, decodeMode string) {
	dbMu.Lock()
	defer dbMu.Unlock()

	if loadedDBPath != "" && loadedDBPath != path {
		_ = os.Remove(loadedDBPath)
	}
	loadedDBPath = path
	loadedEntries = entries
	loadedDecodeMode = decodeMode
}

func getLoadedDB() (path string, entries []dbEntry, decodeMode string) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	path = loadedDBPath
	decodeMode = loadedDecodeMode

	if loadedEntries == nil {
		return path, nil, decodeMode
	}
	cp := make([]dbEntry, len(loadedEntries))
	copy(cp, loadedEntries)
	return path, cp, decodeMode
}

// =====================
// Similarity (Bytes + SHA256 lossy) - SEM normalizar
// =====================

const maxBits = 128

func stringParaBits(texto string, maxBits int) []byte {
	bits := make([]byte, 0, maxBits)
	b := []byte(texto)

	for _, by := range b {
		for i := 7; i >= 0; i-- {
			bits = append(bits, byte((by>>uint(i))&1))
			if len(bits) == maxBits {
				return bits
			}
		}
	}

	for len(bits) < maxBits {
		bits = append(bits, 0)
	}
	return bits
}

func similaridadeBits(a, b []byte) float64 {
	k := len(a)
	if len(b) < k {
		k = len(b)
	}
	if k == 0 {
		return 0.0
	}
	iguais := 0
	for i := 0; i < k; i++ {
		if a[i] == b[i] {
			iguais++
		}
	}
	return float64(iguais) / float64(k)
}

// igual Rust: String::from_utf8_lossy(&hash_bytes)
func utf8LossyString(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))

	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			sb.WriteRune(utf8.RuneError)
			b = b[1:]
			continue
		}
		sb.WriteRune(r)
		b = b[size:]
	}
	return sb.String()
}

func bitsHashOf(texto string) []byte {
	h := sha256.Sum256([]byte(texto))
	los := utf8LossyString(h[:])
	return stringParaBits(los, maxBits)
}

// =====================
// Core: escolhe vencedor por max(BytesPct, Sha256Pct)
// =====================

func checkBrand(query string, entries []dbEntry, binPath string, decodeMode string, topN int) CheckResult {
	qBits := stringParaBits(query, maxBits)
	qHashBits := bitsHashOf(query)

	bestWinner := -1.0
	var best *Candidate
	top := make([]Candidate, 0, 16)

	for _, e := range entries {
		bytesPct := similaridadeBits(qBits, e.BitsText) * 100.0
		shaPct := similaridadeBits(qHashBits, e.BitsHash) * 100.0

		winner := bytesPct
		if shaPct > winner {
			winner = shaPct
		}

		c := Candidate{
			Nome:      e.Name,
			BytesPct:  bytesPct,
			Sha256Pct: shaPct,
			WinnerPct: winner,
			Sha256Hex: e.Sha256Hex,
		}

		if best == nil || winner > bestWinner {
			bestWinner = winner
			tmp := c
			best = &tmp
		}

		top = append(top, c)
	}

	sort.Slice(top, func(i, j int) bool {
		if top[i].WinnerPct != top[j].WinnerPct {
			return top[i].WinnerPct > top[j].WinnerPct
		}
		if top[i].BytesPct != top[j].BytesPct {
			return top[i].BytesPct > top[j].BytesPct
		}
		if top[i].Sha256Pct != top[j].Sha256Pct {
			return top[i].Sha256Pct > top[j].Sha256Pct
		}
		return top[i].Nome < top[j].Nome
	})

	if topN <= 0 {
		topN = 5
	}
	if topN > 25 {
		topN = 25
	}
	if len(top) > topN {
		top = top[:topN]
	}

	msg := "Métricas calculadas. Nenhuma decisão aplicada."
	if len(entries) == 0 {
		msg = "Nenhuma marca no banco."
	}

	return CheckResult{
		Query:       query,
		TotalMarcas: len(entries),
		BinPath:     binPath,
		DecodeMode:  decodeMode,
		Best:        best,
		Top:         top,
		Message:     msg,
	}
}

// =====================
// TERRAMIN loader (estrito)
// =====================

// header: "TERRAMIN"(8) + u32 + 4*f64 + 3*u32 + 2*u64 = 72 bytes
// depois: sig32 = sha256(header + payload)
// payload records: [u32 len][utf8 text][4*f64][u64 ts]

var errInvalidTerramin = errors.New("bin inválido (esperado TERRAMIN + assinatura SHA256 válida)")

func parseTerraminWithHeaderLen(data []byte, headerLen int) ([]dbEntry, error) {
	if len(data) < headerLen+32 {
		return nil, errInvalidTerramin
	}
	if string(data[:8]) != "TERRAMIN" {
		return nil, errInvalidTerramin
	}

	header := data[:headerLen]
	sig := data[headerLen : headerLen+32]
	payload := data[headerLen+32:]

	// assinatura: sha256(header + payload)
	h := sha256.New()
	_, _ = h.Write(header)
	_, _ = h.Write(payload)
	sum := h.Sum(nil)
	if !bytesEqual(sum, sig) {
		return nil, errInvalidTerramin
	}

	seen := map[string]bool{}
	out := make([]dbEntry, 0, 1024)

	i := 0
	for {
		if i == len(payload) {
			break
		}
		if i+4 > len(payload) {
			return nil, errInvalidTerramin
		}

		n := int(binary.LittleEndian.Uint32(payload[i : i+4]))
		i += 4

		if n <= 0 || n > 1_000_000 || i+n > len(payload) {
			return nil, errInvalidTerramin
		}

		txtBytes := payload[i : i+n]
		i += n

		if !utf8.Valid(txtBytes) {
			return nil, errInvalidTerramin
		}

		// pular 4*f64 + u64
		if i+40 > len(payload) {
			return nil, errInvalidTerramin
		}
		i += 40

		name := strings.TrimSpace(string(txtBytes))
		if name == "" {
			continue
		}
		if strings.HasPrefix(name, "#") || strings.HasPrefix(name, "//") {
			continue
		}
		if len(name) > 200 {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true

		hname := sha256.Sum256([]byte(name))
		out = append(out, dbEntry{
			Name:      name,
			BitsText:  stringParaBits(name, maxBits),
			BitsHash:  bitsHashOf(name),
			Sha256Hex: hex.EncodeToString(hname[:]),
		})
	}

	if len(out) == 0 {
		return nil, errInvalidTerramin
	}
	return out, nil
}

func loadEntriesFromFile(path string) ([]dbEntry, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", errInvalidTerramin
	}

	// tenta 72 (correto). se falhar, tenta 76 (compat)
	if entries, err := parseTerraminWithHeaderLen(data, 72); err == nil {
		return entries, "terramin_v1", nil
	}
	if entries, err := parseTerraminWithHeaderLen(data, 76); err == nil {
		return entries, "terramin_v1_header76", nil
	}

	return nil, "", errInvalidTerramin
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// =====================
// HTTP server
// =====================

func main() {
	projectRoot, _ := os.Getwd()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		indexPath := filepath.Join(projectRoot, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Coloque um index.html ao lado do server e abra / (ou use /load_marcas e /check_brand)."))
	})

	// POST /load_marcas (multipart field: "marcas")
	http.HandleFunc("/load_marcas", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		_ = r.ParseMultipartForm(64 << 20)
		mf, _, err := r.FormFile("marcas")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, LoadDBResponse{
				Ok:    false,
				Mode:  "file_missing",
				Error: "arquivo nao enviado (campo 'marcas')",
			})
			return
		}
		defer mf.Close()

		tmp, err := os.CreateTemp("", "td_marcas_*.bin")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, LoadDBResponse{
				Ok:    false,
				Mode:  "temp_fail",
				Error: "falha ao criar temp",
			})
			return
		}

		n, copyErr := io.Copy(tmp, mf)
		_ = tmp.Close()

		if copyErr != nil {
			_ = os.Remove(tmp.Name())
			writeJSON(w, http.StatusInternalServerError, LoadDBResponse{
				Ok:    false,
				Size:  n,
				Mode:  "save_fail",
				Error: "falha ao salvar arquivo",
			})
			return
		}

		entries, decodeMode, derr := loadEntriesFromFile(tmp.Name())
		if derr != nil {
			_ = os.Remove(tmp.Name())
			writeJSON(w, http.StatusBadRequest, LoadDBResponse{
				Ok:    false,
				Size:  n,
				Mode:  "bin_invalid",
				Error: derr.Error(),
			})
			return
		}

		setLoadedDB(tmp.Name(), entries, decodeMode)

		writeJSON(w, http.StatusOK, LoadDBResponse{
			Ok:          true,
			Size:        n,
			Mode:        "loaded",
			TotalMarcas: len(entries),
			DecodeMode:  decodeMode,
		})
	})

	// POST /check_brand body: {"brand":"...", "top_n": 10}
	http.HandleFunc("/check_brand", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CheckBrandRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		query := strings.TrimSpace(req.Brand)
		if query == "" {
			writeJSON(w, http.StatusBadRequest, CheckBrandResponse{
				Mode:  "empty_brand",
				Error: "brand vazio",
			})
			return
		}

		path, entries, decodeMode := getLoadedDB()
		if path == "" || len(entries) == 0 {
			writeJSON(w, http.StatusBadRequest, CheckBrandResponse{
				Mode:  "db_not_loaded",
				Error: "marcas.bin nao carregado (use /load_marcas)",
			})
			return
		}

		report := checkBrand(query, entries, path, decodeMode, req.TopN)

		writeJSON(w, http.StatusOK, CheckBrandResponse{
			Mode:   "ok",
			Report: &report,
		})
	})

	log.Printf("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
