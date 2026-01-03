package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
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
	Base64Pct float64 `json:"base64_pct"`

	LevPct   float64 `json:"lev_pct"`
	TriPct   float64 `json:"tri_pct"`
	Poly2Pct float64 `json:"poly2_pct"`

	WinnerPct float64 `json:"winner_pct"`
	CombPct   float64 `json:"comb_pct"`
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
	BitsB64   []byte
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
		if isTempDBFile(loadedDBPath) {
			_ = os.Remove(loadedDBPath)
		}
	}

	loadedDBPath = path
	loadedEntries = entries
	loadedDecodeMode = decodeMode
}

func isTempDBFile(p string) bool {
	base := filepath.Base(p)
	tmp := os.TempDir()
	return strings.HasPrefix(base, "td_marcas_") && strings.HasPrefix(strings.ToLower(p), strings.ToLower(tmp))
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
// Similarity - IDÊNTICO ao Rust (sua base)
// =====================

const maxBits = 128

func stringParaBits(texto string, maxBits int) []byte {
	bits := make([]byte, 0, maxBits*2)
	for _, by := range []byte(texto) {
		for i := 7; i >= 0; i-- {
			bits = append(bits, byte((by>>uint(i))&1))
		}
	}
	if len(bits) > maxBits {
		bits = bits[:maxBits]
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

// Replica exatamente Rust: String::from_utf8_lossy(&bytes)
func utf8LossyString(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))

	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			sb.WriteRune('�') // U+FFFD replacement character
			b = b[1:]
			continue
		}
		sb.WriteRune(r)
		b = b[size:]
	}
	return sb.String()
}

func similaridadeSHA256(a, b string) float64 {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))

	ba := stringParaBits(utf8LossyString(ha[:]), maxBits)
	bb := stringParaBits(utf8LossyString(hb[:]), maxBits)

	return similaridadeBits(ba, bb)
}

func bitsHashOfLossy(texto string) []byte {
	sum := sha256.Sum256([]byte(texto))
	los := utf8LossyString(sum[:])
	return stringParaBits(los, maxBits)
}

func bitsBase64Of(texto string) []byte {
	b64 := base64.StdEncoding.EncodeToString([]byte(texto))
	return stringParaBits(b64, maxBits)
}

// =====================
// POLINOMIAIS: LEV + TRI + POLY2
// =====================

func canonText(s string) string {
	lower := strings.ToLower(s)
	var out strings.Builder
	out.Grow(len(lower))

	lastSpace := true

	for _, ch := range lower {
		c := ch
		switch ch {
		case 'á', 'à', 'ã', 'â', 'ä':
			c = 'a'
		case 'é', 'ê', 'è', 'ë':
			c = 'e'
		case 'í', 'ì', 'î', 'ï':
			c = 'i'
		case 'ó', 'ô', 'õ', 'ò', 'ö':
			c = 'o'
		case 'ú', 'ù', 'û', 'ü':
			c = 'u'
		case 'ç':
			c = 'c'
		}

		isAZ := (c >= 'a' && c <= 'z')
		is09 := (c >= '0' && c <= '9')

		if isAZ || is09 {
			out.WriteRune(c)
			lastSpace = false
		} else {
			if !lastSpace {
				out.WriteByte(' ')
				lastSpace = true
			}
		}
	}

	return strings.TrimSpace(out.String())
}

// distância Levenshtein em bytes (após canonizar vira ASCII)
func levenshteinDistance(a, b string) int {
	ab := []byte(a)
	bb := []byte(b)

	if len(ab) == 0 {
		return len(bb)
	}
	if len(bb) == 0 {
		return len(ab)
	}

	dp := make([]int, len(bb)+1)
	for j := 0; j <= len(bb); j++ {
		dp[j] = j
	}

	for i := 0; i < len(ab); i++ {
		prev := dp[0]
		dp[0] = i + 1

		for j := 0; j < len(bb); j++ {
			tmp := dp[j+1]
			cost := 0
			if ab[i] != bb[j] {
				cost = 1
			}

			del := dp[j+1] + 1
			ins := dp[j] + 1
			sub := prev + cost

			dp[j+1] = del
			if ins < dp[j+1] {
				dp[j+1] = ins
			}
			if sub < dp[j+1] {
				dp[j+1] = sub
			}

			prev = tmp
		}
	}

	return dp[len(bb)]
}

func similaridadeLEV(a, b string) float64 {
	ca := canonText(a)
	cb := canonText(b)

	maxLen := len(ca)
	if len(cb) > maxLen {
		maxLen = len(cb)
	}
	if maxLen == 0 {
		return 1.0
	}

	d := float64(levenshteinDistance(ca, cb))
	s := 1.0 - (d / float64(maxLen))
	if s < 0.0 {
		return 0.0
	}
	if s > 1.0 {
		return 1.0
	}
	return s
}

// TRI (Jaccard de trigramas)
func trigramSet(s string) map[string]struct{} {
	cs := canonText(s)
	set := make(map[string]struct{})

	if cs == "" {
		return set
	}
	if len(cs) < 3 {
		set[cs] = struct{}{}
		return set
	}

	for i := 0; i <= len(cs)-3; i++ {
		g := cs[i : i+3]
		set[g] = struct{}{}
	}
	return set
}

func similaridadeTRI(a, b string) float64 {
	sa := trigramSet(a)
	sb := trigramSet(b)

	if len(sa) == 0 && len(sb) == 0 {
		return 1.0
	}
	if len(sa) == 0 || len(sb) == 0 {
		return 0.0
	}

	// inter
	inter := 0
	if len(sa) < len(sb) {
		for k := range sa {
			if _, ok := sb[k]; ok {
				inter++
			}
		}
	} else {
		for k := range sb {
			if _, ok := sa[k]; ok {
				inter++
			}
		}
	}

	uni := len(sa) + len(sb) - inter
	if uni <= 0 {
		return 0.0
	}
	return float64(inter) / float64(uni)
}

// POLY2 (kernel polinomial normalizado em trigramas, grau 2)
func trigramCounts(s string) map[string]uint32 {
	cs := canonText(s)
	m := make(map[string]uint32)

	if cs == "" {
		return m
	}
	if len(cs) < 3 {
		m[cs] = 1
		return m
	}

	for i := 0; i <= len(cs)-3; i++ {
		g := cs[i : i+3]
		m[g]++
	}
	return m
}

func similaridadePOLY2(a, b string) float64 {
	va := trigramCounts(a)
	vb := trigramCounts(b)

	if len(va) == 0 && len(vb) == 0 {
		return 1.0
	}
	if len(va) == 0 || len(vb) == 0 {
		return 0.0
	}

	var dot uint64
	for k, ca := range va {
		if cb, ok := vb[k]; ok {
			dot += uint64(ca) * uint64(cb)
		}
	}

	var aa uint64
	for _, c := range va {
		aa += uint64(c) * uint64(c)
	}

	var bb uint64
	for _, c := range vb {
		bb += uint64(c) * uint64(c)
	}

	// num=(dot+1)^2
	num := float64(dot + 1)
	num = num * num

	// den=(aa+1)(bb+1)
	den := float64(aa+1) * float64(bb+1)
	if den <= 0.0 {
		return 0.0
	}

	s := num / den
	if s < 0.0 {
		return 0.0
	}
	if s > 1.0 {
		return 1.0
	}
	return s
}

// =====================
// Core: Winner = média geral de TUDO
// Winner = (Bytes + SHA + Base64 + LEV + TRI + POLY2) / 6
// =====================

func checkBrand(query string, entries []dbEntry, binPath string, decodeMode string, topN int) CheckResult {
	qBits := stringParaBits(query, maxBits)
	qB64Bits := bitsBase64Of(query)

	bestWinner := -1.0
	var best *Candidate
	top := make([]Candidate, 0, 64)

	for _, e := range entries {
		sBytes := similaridadeBits(qBits, e.BitsText) // 0..1
		sSha := similaridadeSHA256(query, e.Name)     // 0..1
		sB64 := similaridadeBits(qB64Bits, e.BitsB64) // 0..1

		comb := (sBytes * 0.3) + (sSha * 0.3) + (sB64 * 0.4)

		sLev := similaridadeLEV(query, e.Name)       // 0..1
		sTri := similaridadeTRI(query, e.Name)       // 0..1
		sP2 := similaridadePOLY2(query, e.Name)      // 0..1
		winner := (sBytes + sSha + sB64 + sLev + sTri + sP2) / 6.0

		c := Candidate{
			Nome:      e.Name,
			BytesPct:  sBytes * 100.0,
			Sha256Pct: sSha * 100.0,
			Base64Pct: sB64 * 100.0,

			LevPct:   sLev * 100.0,
			TriPct:   sTri * 100.0,
			Poly2Pct: sP2 * 100.0,

			CombPct:   comb * 100.0,
			WinnerPct: winner * 100.0,
			Sha256Hex: e.Sha256Hex,
		}

		if best == nil || winner > bestWinner {
			bestWinner = winner
			tmp := c
			best = &tmp
		}

		top = append(top, c)
	}

	// Rank por WinnerPct (média geral), depois desempates
	sort.Slice(top, func(i, j int) bool {
		if top[i].WinnerPct != top[j].WinnerPct {
			return top[i].WinnerPct > top[j].WinnerPct
		}
		if top[i].LevPct != top[j].LevPct {
			return top[i].LevPct > top[j].LevPct
		}
		if top[i].TriPct != top[j].TriPct {
			return top[i].TriPct > top[j].TriPct
		}
		if top[i].Poly2Pct != top[j].Poly2Pct {
			return top[i].Poly2Pct > top[j].Poly2Pct
		}
		if top[i].CombPct != top[j].CombPct {
			return top[i].CombPct > top[j].CombPct
		}
		if top[i].Sha256Pct != top[j].Sha256Pct {
			return top[i].Sha256Pct > top[j].Sha256Pct
		}
		if top[i].BytesPct != top[j].BytesPct {
			return top[i].BytesPct > top[j].BytesPct
		}
		if top[i].Base64Pct != top[j].Base64Pct {
			return top[i].Base64Pct > top[j].Base64Pct
		}
		return top[i].Nome < top[j].Nome
	})

	if topN <= 0 {
		topN = 10
	}
	if topN > 25 {
		topN = 25
	}
	if len(top) > topN {
		top = top[:topN]
	}

	msg := "WinnerPct = média( BytesPct, Sha256Pct, Base64Pct, LevPct, TriPct, Poly2Pct ). (CombPct mantido só para diagnóstico)"

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
// TERRAMIN loader
// =====================

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

	h := sha256.New()
	_, _ = h.Write(header)
	_, _ = h.Write(payload)
	sum := h.Sum(nil)
	if !bytes.Equal(sum, sig) {
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
			BitsHash:  bitsHashOfLossy(name),
			BitsB64:   bitsBase64Of(name),
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

	if entries, err := parseTerraminWithHeaderLen(data, 72); err == nil {
		return entries, "terramin_v1", nil
	}
	if entries, err := parseTerraminWithHeaderLen(data, 76); err == nil {
		return entries, "terramin_v1_header76", nil
	}

	return nil, "", errInvalidTerramin
}

// =====================
// Helpers
// =====================

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
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
		http.ServeFile(w, r, indexPath)
	})

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
				Error: "marcas.bin nao carregado (use o botão 'Carregar base' no HTML)",
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
