package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

//
// =====================
// API DTOs
// =====================
//

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
}

type CheckBrandResponse struct {
	Mode   string       `json:"mode"`
	Report *CheckResult `json:"report,omitempty"`
	Error  string       `json:"error,omitempty"`
}

//
// =====================
// Report (SEM score / SEM média)
// - escolhe o candidato cujo max(BytesPct, Sha256Pct) é o maior
// - retorna APENAS nome + bytes_pct + sha256_pct do vencedor
// =====================
//

type BestMatch struct {
	Nome      string  `json:"nome"`
	BytesPct  float64 `json:"bytes_pct"`
	Sha256Pct float64 `json:"sha256_pct"`
}

type CheckResult struct {
	Query       string     `json:"query"`
	TotalMarcas int        `json:"total_marcas"`
	BinPath     string     `json:"bin_path"`
	DecodeMode  string     `json:"decode_mode"`
	Best        *BestMatch `json:"best,omitempty"`
	Message     string     `json:"message"`
}

//
// =====================
// State
// =====================
//

var dbMu sync.RWMutex
var loadedDBPath string
var loadedNames []string
var loadedDecodeMode string

func setLoadedDB(path string, names []string, decodeMode string) {
	dbMu.Lock()
	defer dbMu.Unlock()

	if loadedDBPath != "" && loadedDBPath != path {
		_ = os.Remove(loadedDBPath)
	}
	loadedDBPath = path
	loadedNames = names
	loadedDecodeMode = decodeMode
}

func getLoadedDB() (path string, names []string, decodeMode string) {
	dbMu.RLock()
	defer dbMu.RUnlock()

	path = loadedDBPath
	decodeMode = loadedDecodeMode
	if loadedNames == nil {
		return path, nil, decodeMode
	}
	cp := make([]string, len(loadedNames))
	copy(cp, loadedNames)
	return path, cp, decodeMode
}

//
// =====================
// Canonicalização: REMOVER ACENTOS (só para comparar)
// - NÃO muda o nome original retornado
// =====================
//

var accentReplacer = strings.NewReplacer(
	// a
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "å", "a",
	"Á", "A", "À", "A", "Â", "A", "Ã", "A", "Ä", "A", "Å", "A",
	// e
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"É", "E", "È", "E", "Ê", "E", "Ë", "E",
	// i
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"Í", "I", "Ì", "I", "Î", "I", "Ï", "I",
	// o
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"Ó", "O", "Ò", "O", "Ô", "O", "Õ", "O", "Ö", "O",
	// u
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"Ú", "U", "Ù", "U", "Û", "U", "Ü", "U",
	// c
	"ç", "c", "Ç", "C",
	// n (p/ espanhol/nomes)
	"ñ", "n", "Ñ", "N",
	// y
	"ý", "y", "ÿ", "y",
	"Ý", "Y",
)

func stripAccents(s string) string {
	// remove acentos, mantendo o resto intacto (inclusive caixa/case)
	return accentReplacer.Replace(s)
}

//
// =====================
// Similarity (Bytes + SHA256 lossy, igual seu estilo)
// =====================
//

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

func similaridadeBytes(a, b string) float64 {
	ba := stringParaBits(a, 128)
	bb := stringParaBits(b, 128)
	return similaridadeBits(ba, bb)
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

func similaridadeSHA256(a, b string) float64 {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))

	sa := utf8LossyString(ha[:])
	sb := utf8LossyString(hb[:])

	ba := stringParaBits(sa, 128)
	bb := stringParaBits(sb, 128)

	return similaridadeBits(ba, bb)
}

//
// =====================
// Core (REGRA QUE VOCÊ MANDOU)
// - remove acento SOMENTE para comparar
// - winnerMetric = max(BytesPct, Sha256Pct)
// - retorna as DUAS métricas do vencedor (bytes + sha)
// =====================
//

func checkBrand(query string, names []string, binPath string, decodeMode string) CheckResult {
	var best *BestMatch
	var bestWinner float64

	// ✅ comparar usando versões sem acento
	qCmp := stripAccents(query)

	for _, nm := range names {
		nCmp := stripAccents(nm)

		bytesPct := similaridadeBytes(qCmp, nCmp) * 100.0
		shaPct := similaridadeSHA256(qCmp, nCmp) * 100.0

		winnerMetric := bytesPct
		if shaPct > winnerMetric {
			winnerMetric = shaPct
		}

		if best == nil || winnerMetric > bestWinner {
			bestWinner = winnerMetric
			best = &BestMatch{
				Nome:      nm, // ✅ retorna ORIGINAL
				BytesPct:  bytesPct,
				Sha256Pct: shaPct,
			}
		}
	}

	msg := "Métricas calculadas. Nenhuma decisão aplicada."
	if best == nil {
		msg = "Nenhuma marca no banco."
	}

	return CheckResult{
		Query:       query,
		TotalMarcas: len(names),
		BinPath:     binPath,
		DecodeMode:  decodeMode,
		Best:        best,
		Message:     msg,
	}
}

//
// =====================
// marcas.bin loader (ROBUSTO + CORREÇÃO)
// - scan de offsets até 4096 (não 256)
// - extractASCIIStrings aceita apóstrofo ' (para McDonald's não quebrar)
// - sem filterCandidate (só trim+dedupe)
// =====================
//

type binReader struct {
	b []byte
	i int
}

func (r *binReader) remaining() int { return len(r.b) - r.i }

func (r *binReader) readU64LE() (uint64, bool) {
	if r.remaining() < 8 {
		return 0, false
	}
	v := binary.LittleEndian.Uint64(r.b[r.i : r.i+8])
	r.i += 8
	return v, true
}

func (r *binReader) readVarU64() (uint64, bool) {
	v, n := binary.Uvarint(r.b[r.i:])
	if n <= 0 {
		return 0, false
	}
	r.i += n
	return v, true
}

func (r *binReader) readBytes(n int) ([]byte, bool) {
	if n < 0 || r.remaining() < n {
		return nil, false
	}
	out := r.b[r.i : r.i+n]
	r.i += n
	return out, true
}

func looksReasonableCount(cnt uint64, remaining int) bool {
	if cnt == 0 {
		return true
	}
	if cnt > 1_000_000 {
		return false
	}
	if int(cnt) > remaining {
		return false
	}
	return true
}

func readBincodeStringFixint(r *binReader) (string, bool) {
	n, ok := r.readU64LE()
	if !ok {
		return "", false
	}
	if n > uint64(r.remaining()) || n > 1_000_000 {
		return "", false
	}
	b, ok := r.readBytes(int(n))
	if !ok || !utf8.Valid(b) {
		return "", false
	}
	return string(b), true
}

func readBincodeStringVarint(r *binReader) (string, bool) {
	n, ok := r.readVarU64()
	if !ok {
		return "", false
	}
	if n > uint64(r.remaining()) || n > 1_000_000 {
		return "", false
	}
	b, ok := r.readBytes(int(n))
	if !ok || !utf8.Valid(b) {
		return "", false
	}
	return string(b), true
}

// Vec<Marca> fixint: [u64 cnt][String nome][String hash][u64 data][String tx]...
func tryVecMarcaFixint(payload []byte) ([]string, bool) {
	r := &binReader{b: payload}
	cnt, ok := r.readU64LE()
	if !ok || !looksReasonableCount(cnt, r.remaining()) {
		return nil, false
	}
	out := make([]string, 0, cnt)
	for i := uint64(0); i < cnt; i++ {
		nome, ok := readBincodeStringFixint(r)
		if !ok {
			return nil, false
		}
		if _, ok := readBincodeStringFixint(r); !ok { // hash
			return nil, false
		}
		if _, ok := r.readU64LE(); !ok { // data
			return nil, false
		}
		if _, ok := readBincodeStringFixint(r); !ok { // tx
			return nil, false
		}
		out = append(out, nome)
	}
	return out, true
}

// Vec<Marca> varint
func tryVecMarcaVarint(payload []byte) ([]string, bool) {
	r := &binReader{b: payload}
	cnt, ok := r.readVarU64()
	if !ok || !looksReasonableCount(cnt, r.remaining()) {
		return nil, false
	}
	out := make([]string, 0, cnt)
	for i := uint64(0); i < cnt; i++ {
		nome, ok := readBincodeStringVarint(r)
		if !ok {
			return nil, false
		}
		if _, ok := readBincodeStringVarint(r); !ok { // hash
			return nil, false
		}
		if _, ok := r.readVarU64(); !ok { // data
			return nil, false
		}
		if _, ok := readBincodeStringVarint(r); !ok { // tx
			return nil, false
		}
		out = append(out, nome)
	}
	return out, true
}

// Vec<String> fixint
func tryVecStringFixint(payload []byte) ([]string, bool) {
	r := &binReader{b: payload}
	cnt, ok := r.readU64LE()
	if !ok || !looksReasonableCount(cnt, r.remaining()) {
		return nil, false
	}
	out := make([]string, 0, cnt)
	for i := uint64(0); i < cnt; i++ {
		s, ok := readBincodeStringFixint(r)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// Vec<String> varint
func tryVecStringVarint(payload []byte) ([]string, bool) {
	r := &binReader{b: payload}
	cnt, ok := r.readVarU64()
	if !ok || !looksReasonableCount(cnt, r.remaining()) {
		return nil, false
	}
	out := make([]string, 0, cnt)
	for i := uint64(0); i < cnt; i++ {
		s, ok := readBincodeStringVarint(r)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

func cleanNames(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if len(s) > 200 {
			continue
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func extractASCIIStrings(data []byte) []string {
	seen := map[string]bool{}
	out := []string{}

	var sb strings.Builder
	flush := func() {
		if sb.Len() == 0 {
			return
		}
		s := strings.TrimSpace(sb.String())
		sb.Reset()
		if s == "" {
			return
		}
		if len(s) > 200 {
			return
		}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}

	for i := 0; i < len(data); i++ {
		c := data[i]
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == ' ' || c == '-' || c == '_' || c == '.' || c == '&' ||
			c == '+' || c == '/' || c == '\'' { // ✅ apóstrofo para McDonald's
			sb.WriteByte(c)
			if sb.Len() > 240 {
				flush()
			}
			continue
		}
		flush()
	}
	flush()
	return out
}

type decodeCandidate struct {
	names      []string
	mode       string
	scoreNames int
}

func decodeAt(payload []byte) *decodeCandidate {
	if names, ok := tryVecMarcaFixint(payload); ok && len(names) > 0 {
		n := cleanNames(names)
		if len(n) > 0 {
			return &decodeCandidate{names: n, mode: "bincode_vec_marca_fixint", scoreNames: len(n)}
		}
	}
	if names, ok := tryVecMarcaVarint(payload); ok && len(names) > 0 {
		n := cleanNames(names)
		if len(n) > 0 {
			return &decodeCandidate{names: n, mode: "bincode_vec_marca_varint", scoreNames: len(n)}
		}
	}
	if names, ok := tryVecStringFixint(payload); ok && len(names) > 0 {
		n := cleanNames(names)
		if len(n) > 0 {
			return &decodeCandidate{names: n, mode: "bincode_vec_string_fixint", scoreNames: len(n)}
		}
	}
	if names, ok := tryVecStringVarint(payload); ok && len(names) > 0 {
		n := cleanNames(names)
		if len(n) > 0 {
			return &decodeCandidate{names: n, mode: "bincode_vec_string_varint", scoreNames: len(n)}
		}
	}
	return nil
}

func loadNamesFromFile(path string) ([]string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", errors.New("arquivo vazio")
	}

	// JSON array (opcional)
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) {
		var arr []any
		if err := json.Unmarshal(data, &arr); err == nil {
			out := []string{}
			for _, it := range arr {
				switch v := it.(type) {
				case string:
					out = append(out, v)
				case map[string]any:
					if s, ok := v["nome"].(string); ok {
						out = append(out, s)
					}
				}
			}
			out = cleanNames(out)
			if len(out) > 0 {
				return out, "json_array", nil
			}
		}
	}

	// Se começar com TERRAMIN, tentar offsets comuns primeiro
	startOffsets := []int{0}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte("TERRAMIN")) {
		startOffsets = []int{64, 32, 16, 12, 8, 0}
	}

	var best *decodeCandidate

	// ✅ varre offsets 0..4096
	maxScan := 4096
	if len(data) < maxScan {
		maxScan = len(data)
	}

	checked := map[int]bool{}
	for _, off := range startOffsets {
		if off >= 0 && off < len(data) {
			checked[off] = true
			if c := decodeAt(data[off:]); c != nil {
				if best == nil || c.scoreNames > best.scoreNames {
					best = c
				}
			}
		}
	}

	for off := 0; off < maxScan; off++ {
		if checked[off] {
			continue
		}
		if c := decodeAt(data[off:]); c != nil {
			if best == nil || c.scoreNames > best.scoreNames {
				best = c
			}
		}
	}

	if best != nil && len(best.names) > 0 {
		return best.names, best.mode, nil
	}

	// fallback: extrair ASCII (último recurso)
	names := extractASCIIStrings(data)
	if len(names) > 0 {
		return cleanNames(names), "fallback_ascii_extract", nil
	}

	return nil, "", errors.New("nao consegui decodificar marcas.bin (TERRAMIN/bincode/json)")
}

//
// =====================
// HTTP server
// =====================
//

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
		marcasPath := filepath.Join(projectRoot, "marcas.html")
		if _, err := os.Stat(marcasPath); err == nil {
			http.ServeFile(w, r, marcasPath)
			return
		}

		http.Error(w, "Nao achei index.html nem marcas.html no diretorio do server.", http.StatusNotFound)
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
			http.Error(w, "arquivo nao enviado (campo 'marcas')", http.StatusBadRequest)
			return
		}
		defer mf.Close()

		tmp, err := os.CreateTemp("", "td_marcas_*.bin")
		if err != nil {
			http.Error(w, "falha ao criar temp: "+err.Error(), http.StatusInternalServerError)
			return
		}

		n, copyErr := io.Copy(tmp, mf)
		_ = tmp.Close()
		if copyErr != nil {
			_ = os.Remove(tmp.Name())
			http.Error(w, "falha ao salvar arquivo: "+copyErr.Error(), http.StatusInternalServerError)
			return
		}

		names, decodeMode, derr := loadNamesFromFile(tmp.Name())
		if derr != nil {
			_ = os.Remove(tmp.Name())
			http.Error(w, "erro ao carregar marcas.bin: "+derr.Error(), http.StatusBadRequest)
			return
		}

		setLoadedDB(tmp.Name(), names, decodeMode)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoadDBResponse{
			Ok:          true,
			Size:        n,
			Mode:        "loaded",
			TotalMarcas: len(names),
			DecodeMode:  decodeMode,
		})
	})

	// POST /check_brand  body: {"brand":"..."}
	http.HandleFunc("/check_brand", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CheckBrandRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		query := strings.TrimSpace(req.Brand)
		if query == "" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(CheckBrandResponse{
				Mode:  "empty_brand",
				Error: "brand vazio",
			})
			return
		}

		path, names, decodeMode := getLoadedDB()
		if path == "" || len(names) == 0 {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(CheckBrandResponse{
				Mode:  "db_not_loaded",
				Error: "marcas.bin nao carregado (use /load_marcas)",
			})
			return
		}

		report := checkBrand(query, names, path, decodeMode)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CheckBrandResponse{
			Mode:   "ok",
			Report: &report,
		})
	})

	log.Printf("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
