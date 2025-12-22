package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"
)

// =====================
// DTOs
// =====================

type AskResponse struct {
	Answer string `json:"answer"`
	Recall string `json:"recall"`
	Mode   string `json:"mode"`   // "hibrido_deterministico" | "ia_generativa_pura" | "deterministico_puro"
	Tokens int    `json:"tokens"` // totalTokenCount quando disponível
}

type LoadMindResponse struct {
	Ok   bool  `json:"ok"`
	Size int64 `json:"size"`
	Mode string `json:"mode"`
}

type geminiResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata struct {
		TotalTokenCount int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type geminiErr struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// =====================
// Config helpers
// =====================

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}

func mustCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func firstExisting(paths ...string) (string, bool) {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs, true
		}
	}
	return "", false
}

func fileExistsNonZero(path string) (bool, int64) {
	st, err := os.Stat(path)
	if err != nil {
		return false, 0
	}
	if st.Size() <= 0 {
		return false, st.Size()
	}
	return true, st.Size()
}

func keyFingerprint(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8]) // fingerprint curto (não vaza a chave)
}

func cleanAPIKey(k string) string {
	k = strings.TrimSpace(k)
	k = strings.Trim(k, "\"")
	k = strings.Trim(k, "'")
	return strings.TrimSpace(k)
}

// NORMALIZA NOME DO MODELO (resolve: "models/..." e sufixos acidentais)
func normalizeModelName(m string) string {
	m = strings.TrimSpace(m)
	m = strings.TrimPrefix(m, "models/")
	m = strings.TrimSuffix(m, ":generateContent")
	m = strings.TrimSuffix(m, ":streamGenerateContent")
	return m
}

// =====================
// Prompts
// =====================

func promptComRecall(pergunta, recall string) string {
	return "VOCÊ É TERRA DOURADA.\n" +
		"TODA autoidentificação deve usar primeira pessoa.\n" +
		"Seja direto. Use abstração mínima apenas para dar fluidez ao texto, sem criar novos conceitos.\n\n" +
		"PERGUNTA:\n" + pergunta + "\n\n" +
		"RECALL SOBERANO:\n" + recall + "\n\n" +
		"RESPOSTA:"
}

func promptSemRecall(pergunta string) string {
	return "SISTEMA: TERRA DOURADA.\n" +
		"TAREFA: Responda diretamente ao usuário.\n" +
		"SE faltar contexto, peça contexto de forma curta.\n\n" +
		"ENTRADA:\n" + pergunta + "\n\n" +
		"RESPOSTA:"
}


// =====================
// Gemini (SEM discovery) + retry + key no query (HARDCODED PARA HACKATHON)
// =====================

func chamarGemini(ctx context.Context, model string, prompt string, temperature float64, maxOut int) (string, int, error) {
	// 🔴🔴🔴 HARDCODE CRÍTICO PARA O HACKATHON 🔴🔴🔴
	// Isso garante que funcione independente do Windows/Env
	apiKey := "AIzaSyDyVsXc00X2ZgBST1TC85Kgjr35qr9Q0Gs"
	
	// Limpeza de segurança (caso haja espaços invisíveis)
	apiKey = cleanAPIKey(apiKey)

	if apiKey == "" {
		return "", 0, fmt.Errorf("GEMINI_API_KEY vazia no código")
	}

	if model == "" {
		model = "gemini-2.0-flash"
	}
	model = normalizeModelName(model)

	// IGUAL ao seu curl: key no query param
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent?key=" + neturl.QueryEscape(apiKey)

	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{
					map[string]string{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     temperature,
			"maxOutputTokens": maxOut,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}

	client := &http.Client{Timeout: 25 * time.Second}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
		if err != nil {
			return "", 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		// também manda no header (não atrapalha)
		req.Header.Set("x-goog-api-key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			raw, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var parsed geminiResp
				if err := json.Unmarshal(raw, &parsed); err != nil {
					return "", 0, fmt.Errorf("falha ao parsear resposta do gemini: %w", err)
				}

				var sb strings.Builder
				if len(parsed.Candidates) > 0 {
					for _, p := range parsed.Candidates[0].Content.Parts {
						t := strings.TrimSpace(p.Text)
						if t == "" {
							continue
						}
						if sb.Len() > 0 {
							sb.WriteString("\n")
						}
						sb.WriteString(t)
					}
				}

				out := strings.TrimSpace(sb.String())
				if out == "" {
					out = "(sem resposta)"
				}
				return out, parsed.UsageMetadata.TotalTokenCount, nil
			}

			// tenta parsear erro do Gemini
			var ge geminiErr
			if json.Unmarshal(raw, &ge) == nil && ge.Error.Message != "" {
				// Retry só em rate-limit/instabilidade
				if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 503 || resp.StatusCode == 504 {
					lastErr = fmt.Errorf("gemini %d %s: %s", ge.Error.Code, ge.Error.Status, ge.Error.Message)
				} else {
					return "", 0, fmt.Errorf("gemini %d %s: %s", ge.Error.Code, ge.Error.Status, ge.Error.Message)
				}
			} else {
				// erro bruto
				if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 503 || resp.StatusCode == 504 {
					lastErr = fmt.Errorf("gemini status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
				} else {
					return "", 0, fmt.Errorf("gemini status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
				}
			}
		}

		// backoff curto: 250ms, 500ms, 1000ms
		sleep := time.Duration(250*(1<<(attempt-1))) * time.Millisecond
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return "", 0, ctx.Err()
		}
	}

	return "", 0, lastErr
}

// =====================
// CORS
// =====================

func cors(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// =====================
// Helpers: multipart / zip
// =====================

func getMultipartFile(r *http.Request, field string) (multipart.File, *multipart.FileHeader, error) {
	f, fh, err := r.FormFile(field)
	if err == nil {
		return f, fh, nil
	}
	return nil, nil, err
}

func saveToTempFile(prefix string, data io.Reader) (string, error) {
	tmp, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, data); err != nil {
		return "", err
	}
	return tmp.Name(), nil
}

func zipMindAndReport(mindBytes []byte, report string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w1, err := zw.Create("mind.bin")
	if err != nil {
		_ = zw.Close()
		return nil, err
	}
	if _, err := w1.Write(mindBytes); err != nil {
		_ = zw.Close()
		return nil, err
	}

	w2, err := zw.Create("report.txt")
	if err != nil {
		_ = zw.Close()
		return nil, err
	}
	if report == "" {
		report = "(sem log)"
	}
	if _, err := w2.Write([]byte(report)); err != nil {
		_ = zw.Close()
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// =====================
// Normalização p/ persistencia.rs (sem mudar Rust)
// =====================

func alphaCount(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// binário -> texto seguro: não imprimível vira '\n'
func carveToText(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	for _, b := range raw {
		switch {
		case b == '\n':
			out = append(out, '\n')
		case b == '\r':
			out = append(out, '\n')
		case b == '\t':
			out = append(out, ' ')
		case b >= 0x20 && b <= 0x7E:
			out = append(out, b)
		default:
			out = append(out, '\n')
		}
	}
	return out
}

// garante segmentos que passam no filtro do persistencia.rs: len>=25 e alpha>=12
func buildMindBinForPersistencia(txt []byte) []byte {
	s := string(txt)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\u0000", "\n")

	var sentences []string
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(r)
		if r == '.' || r == '?' || r == '!' || r == '\n' {
			sentences = append(sentences, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		sentences = append(sentences, b.String())
	}

	var entries []string
	var cur strings.Builder

	flush := func() {
		t := strings.TrimSpace(cur.String())
		cur.Reset()
		if t == "" {
			return
		}
		last := t[len(t)-1]
		if last != '.' && last != '?' && last != '!' {
			t += "."
		}
		entries = append(entries, t)
	}

	for _, sen := range sentences {
		t := strings.TrimSpace(sen)
		if t == "" {
			continue
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(t)

		ct := strings.TrimSpace(cur.String())
		if len(ct) >= 25 && alphaCount(ct) >= 12 {
			flush()
		}
	}

	rest := strings.TrimSpace(cur.String())
	if rest != "" {
		if len(entries) == 0 {
			entries = append(entries, rest)
		} else {
			entries[len(entries)-1] = strings.TrimSpace(entries[len(entries)-1] + " " + rest)
		}
	}

	for i := range entries {
		e := strings.TrimSpace(entries[i])
		if e == "" {
			continue
		}
		last := e[len(e)-1]
		if last != '.' && last != '?' && last != '!' {
			e += "."
		}
		entries[i] = e
	}

	out := strings.Join(entries, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out)
}

func normalizeMindBytesForPersistencia(raw []byte) []byte {
	return buildMindBinForPersistencia(carveToText(raw))
}

// =====================
// stdout Rust cleaner (separa logs da resposta)
// =====================

func looksLikeRustLogLine(line string) bool {
	l := strings.TrimSpace(line)
	if l == "" {
		return true
	}
	if strings.Contains(l, "[PERSIST]") || strings.Contains(l, "[LOAD]") {
		return true
	}
	if strings.HasPrefix(l, "🧠") || strings.HasPrefix(l, "📂") || strings.HasPrefix(l, "📦") ||
		strings.HasPrefix(l, "🧪") || strings.HasPrefix(l, "✅") || strings.HasPrefix(l, "⚠️") ||
		strings.HasPrefix(l, "⏳") || strings.HasPrefix(l, "❌") {
		return true
	}
	if strings.HasPrefix(l, "[") && strings.Contains(l, "]") {
		return true
	}
	return false
}

func stdoutToAnswer(stdout string) string {
	stdout = strings.ReplaceAll(stdout, "\r\n", "\n")
	lines := strings.Split(stdout, "\n")

	var kept []string
	for _, line := range lines {
		if looksLikeRustLogLine(line) {
			continue
		}
		t := strings.TrimRight(line, "\r")
		if strings.TrimSpace(t) == "" {
			continue
		}
		kept = append(kept, t)
	}

	if len(kept) > 0 {
		return strings.TrimSpace(strings.Join(kept, "\n"))
	}

	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t != "" {
			return t
		}
	}
	return "(não sei)"
}

// =====================
// Gestão de Mind carregado
// =====================

var mindMu sync.Mutex
var loadedMindPath string

func setLoadedMind(path string) {
	mindMu.Lock()
	defer mindMu.Unlock()
	if loadedMindPath != "" && loadedMindPath != path {
		_ = os.Remove(loadedMindPath)
	}
	loadedMindPath = path
}

func getLoadedMind() string {
	mindMu.Lock()
	defer mindMu.Unlock()
	return loadedMindPath
}

func clearLoadedMind() {
	mindMu.Lock()
	defer mindMu.Unlock()
	if loadedMindPath != "" {
		_ = os.Remove(loadedMindPath)
	}
	loadedMindPath = ""
}

// =====================
// copyFile (fallback treino escreve em src/data)
// =====================

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	_ = os.MkdirAll(filepath.Dir(dst), 0700)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// =====================
// MAIN
// =====================

func main() {
	// 🔴 HACK: Força a variável pro sistema todo logo no boot
	// Isso faz o log ficar verdinho e bonito
	os.Setenv("GEMINI_API_KEY", "AIzaSyDyVsXc00X2ZgBST1TC85Kgjr35qr9Q0Gs")

	projectRoot := os.Getenv("TD_PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot = mustCwd()
	}

	// Gemini config
	maxOut := envInt("TD_MAX_OUT", 256)
	defaultTemp := envFloat("TD_TEMP", 0.6)

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.0-flash"
	}
	model = normalizeModelName(model)

	// Rust bins
	sfx := exeSuffix()
	treinoBin, okTreino := firstExisting(
		filepath.Join(projectRoot, "target", "release", "treino"+sfx),
		filepath.Join(projectRoot, "target", "release", "treino_terra_dourada"+sfx),
		filepath.Join(projectRoot, "treino"+sfx),
		filepath.Join(projectRoot, "treino_terra_dourada"+sfx),
	)
	recallBin, okRecall := firstExisting(
		filepath.Join(projectRoot, "target", "release", "recall_terra_dourada"+sfx),
		filepath.Join(projectRoot, "target", "release", "recall"+sfx),
		filepath.Join(projectRoot, "recall_terra_dourada"+sfx),
		filepath.Join(projectRoot, "recall"+sfx),
	)

	defaultMind := filepath.Join(projectRoot, "src", "data", "mind.bin")
	defaultResults := filepath.Join(projectRoot, "src", "data", "resultados_fxl.txt")

	apiKey := cleanAPIKey(os.Getenv("GEMINI_API_KEY"))
	if apiKey != "" {
		log.Printf("🔑 GEMINI_API_KEY ok (len=%d, fp=%s)", len(apiKey), keyFingerprint(apiKey))
	} else {
		log.Printf("⚠️ GEMINI_API_KEY não definida (processo do Go não recebeu a env)")
	}

	log.Printf("🚀 Terra Dourada server :8080")
	log.Printf("📂 ProjectRoot: %s", projectRoot)
	log.Printf("🤖 GEMINI_MODEL: %s", model)
	if okTreino {
		log.Printf("⚙️ treino: %s", treinoBin)
	} else {
		log.Printf("⚠️ treino: não encontrado")
	}
	if okRecall {
		log.Printf("🧠 recall: %s", recallBin)
	} else {
		log.Printf("⚠️ recall: não encontrado (ok, /ask vira Gemini puro)")
	}

	// "/" → index.html (se existir)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		index := filepath.Join(projectRoot, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, "index.html não encontrado", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, index)
	})

	// "/train"
	http.HandleFunc("/train", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Método inválido", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Erro lendo body", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()

		jobDir, err := os.MkdirTemp("", "td_job_*")
		if err != nil {
			http.Error(w, "Erro criando job temp", http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(jobDir)

		txtPath := filepath.Join(jobDir, "upload.txt")
		mindJobPath := filepath.Join(jobDir, "mind.bin")
		resultJobPath := filepath.Join(jobDir, "resultados.txt")

		if err := os.WriteFile(txtPath, body, 0600); err != nil {
			http.Error(w, "Erro salvando upload.txt", http.StatusInternalServerError)
			return
		}

		report := "(treino não executado)"
		if okTreino {
			log.Println("⚙️ Treino (Rust) iniciando...")
			var out bytes.Buffer

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			cmd := exec.CommandContext(ctx, treinoBin, txtPath)
			cmd.Dir = projectRoot
			cmd.Stdout = &out
			cmd.Stderr = &out
			cmd.Env = append(os.Environ(),
				"TD_MIND_PATH="+mindJobPath,
				"TD_RESULT_PATH="+resultJobPath,
			)

			start := time.Now()
			runErr := cmd.Run()
			dur := time.Since(start)

			report = fmt.Sprintf("Treino: %s\n\n%s", dur.Truncate(time.Millisecond), out.String())
			if runErr != nil {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Falha no treino\n\n" + report))
				return
			}
		}

		var rawMind []byte

		if ok, _ := fileExistsNonZero(mindJobPath); ok {
			rawMind, _ = os.ReadFile(mindJobPath)
		}
		if len(rawMind) == 0 {
			if ok, _ := fileExistsNonZero(defaultMind); ok {
				rawMind, _ = os.ReadFile(defaultMind)
				_ = copyFile(defaultMind, mindJobPath)
			}
		}
		if len(rawMind) == 0 {
			rawMind = body
		}

		mindBytes := normalizeMindBytesForPersistencia(rawMind)
		if len(mindBytes) == 0 {
			http.Error(w, "Falha ao gerar mind.bin normalizado", http.StatusBadRequest)
			return
		}

		zipBytes, err := zipMindAndReport(mindBytes, report)
		if err != nil {
			http.Error(w, "Erro criando zip", http.StatusInternalServerError)
			return
		}

		_ = os.Remove(defaultMind)
		_ = os.Remove(defaultResults)

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="terra_dourada_mind.zip"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(zipBytes)
	})

	// "/load_mind"
	http.HandleFunc("/load_mind", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Método inválido", http.StatusMethodNotAllowed)
			return
		}

		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "multipart/form-data") {
			http.Error(w, "Envie multipart/form-data com field: mind (arquivo)", http.StatusBadRequest)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 80<<20)
		if err := r.ParseMultipartForm(80 << 20); err != nil {
			http.Error(w, "multipart inválido", http.StatusBadRequest)
			return
		}

		mf, _, err := getMultipartFile(r, "mind")
		if err != nil {
			http.Error(w, "Arquivo mind.bin não enviado (field: mind)", http.StatusBadRequest)
			return
		}
		defer mf.Close()

		mindTemp, err := saveToTempFile("td_loaded_mind_*.bin", mf)
		if err != nil {
			http.Error(w, "Erro salvando mind.bin", http.StatusInternalServerError)
			return
		}

		raw, err := os.ReadFile(mindTemp)
		if err != nil || len(raw) == 0 {
			_ = os.Remove(mindTemp)
			http.Error(w, "mind.bin inválido (não consegui ler)", http.StatusBadRequest)
			return
		}

		norm := normalizeMindBytesForPersistencia(raw)
		if len(norm) == 0 {
			_ = os.Remove(mindTemp)
			http.Error(w, "mind.bin não contém texto utilizável pro persistencia.rs", http.StatusBadRequest)
			return
		}

		if err := os.WriteFile(mindTemp, norm, 0600); err != nil {
			_ = os.Remove(mindTemp)
			http.Error(w, "Erro regravando mind.bin normalizado", http.StatusInternalServerError)
			return
		}

		st, err := os.Stat(mindTemp)
		if err != nil || st.Size() <= 0 {
			_ = os.Remove(mindTemp)
			http.Error(w, "mind.bin normalizado ficou vazio", http.StatusBadRequest)
			return
		}

		setLoadedMind(mindTemp)
		log.Printf("📥 Mind carregado: %s (%d bytes)", mindTemp, st.Size())

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(LoadMindResponse{
			Ok:   true,
			Size: st.Size(),
			Mode: "mind_carregado_normalizado",
		})
	})

	// "/ask"
	http.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Método inválido", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)

		q := ""
		ct := r.Header.Get("Content-Type")
		if strings.Contains(ct, "application/json") {
			var p struct {
				Question string `json:"question"`
			}
			_ = json.NewDecoder(r.Body).Decode(&p)
			q = p.Question
		} else {
			_ = r.ParseMultipartForm(10 << 20)
			q = r.FormValue("question")
		}

		q = strings.TrimSpace(q)
		if q == "" {
			http.Error(w, "Pergunta vazia", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		// Recall opcional
		recallRaw := "(não sei)"
		recallFound := false
		mp := getLoadedMind()

		if okRecall && mp != "" {
			if _, err := os.Stat(mp); err == nil {
				log.Printf("🔍 Recall: %q", q)
				var out bytes.Buffer
				cmd := exec.CommandContext(ctx, recallBin, q)
				cmd.Dir = projectRoot
				cmd.Env = append(os.Environ(), "TD_MIND_PATH="+mp)
				cmd.Stdout = &out
				cmd.Stderr = &out
				if err := cmd.Run(); err != nil {
					log.Printf("❌ Erro no recall: %v", err)
				} else {
					recallRaw = strings.TrimSpace(stdoutToAnswer(out.String()))
					if recallRaw != "" && recallRaw != "(não sei)" && !strings.HasPrefix(recallRaw, "❌") {
						recallFound = true
					} else {
						recallRaw = "(não sei)"
					}
				}
			} else {
				clearLoadedMind()
			}
		}

		var prompt string
		var temp float64
		var mode string

		if recallFound {
			prompt = promptComRecall(q, recallRaw)
		temp = 0.2 // ou 0.3
			mode = "hibrido_deterministico"
		} else {
			prompt = promptSemRecall(q)
			temp = defaultTemp
			mode = "ia_generativa_pura"
		}

		answer, tokens, err := chamarGemini(ctx, model, prompt, temp, maxOut)
		if err != nil {
			log.Printf("🔥 Gemini error: %v", err)

			if recallFound {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_ = json.NewEncoder(w).Encode(AskResponse{
					Answer: strings.TrimSpace(recallRaw),
					Recall: recallRaw,
					Mode:   "deterministico_puro",
					Tokens: 0,
				})
				return
			}

			// SEM recall: devolve o erro real (pra você ver o motivo)
			http.Error(w, "Gemini falhou: "+err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(AskResponse{
			Answer: strings.TrimSpace(answer),
			Recall: recallRaw,
			Mode:   mode,
			Tokens: tokens,
		})
	})

	http.HandleFunc("/clear_mind", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}
		clearLoadedMind()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	srv := &http.Server{
		Addr:              ":8080",
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}