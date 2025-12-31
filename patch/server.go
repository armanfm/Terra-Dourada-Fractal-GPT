package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// =====================
// DTOs
// =====================

type AskResponse struct {
	Answer string `json:"answer"`
	Recall string `json:"recall"`
	Mode   string `json:"mode"`
	Tokens int    `json:"tokens"`
}

type LoadMindResponse struct {
	Ok   bool  `json:"ok"`
	Size int64 `json:"size"`
	Mode string `json:"mode"`
}

// =====================
// Gemini DTO
// =====================

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

// =====================
// Helpers
// =====================

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
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

func cleanAPIKey(k string) string {
	k = strings.TrimSpace(k)
	k = strings.Trim(k, "\"")
	k = strings.Trim(k, "'")
	return strings.TrimSpace(k)
}

func normalizeModelName(m string) string {
	m = strings.TrimSpace(m)
	m = strings.TrimPrefix(m, "models/")
	m = strings.TrimSuffix(m, ":generateContent")
	m = strings.TrimSuffix(m, ":streamGenerateContent")
	return m
}

// =====================
// Question filter (SOBERANO) — Lista A + Lista B
// =====================

var listaA = []string{
	"how much",
	"how many",
	"what",
	"which",
	"when",
	"where",
	"quanto",
	"quantos",
	"qual",
	"quais",
	"quando",
	"onde",
}

var listaB = []string{
	"posso",
	"consigo",
	"dá pra",
	"da pra",
	"é possível",
	"e possivel",
	"can i",
	"am i able to",
}

func splitSentences(text string) []string {
	// Split determinístico por delimitadores de frase.
	// Não recorta palavras; só separa sentenças.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	out := []string{}
	var sb strings.Builder

	for _, r := range text {
		sb.WriteRune(r)
		if r == '.' || r == '?' || r == '!' || r == '\n' {
			s := strings.TrimSpace(sb.String())
			if s != "" {
				out = append(out, s)
			}
			sb.Reset()
		}
	}

	tail := strings.TrimSpace(sb.String())
	if tail != "" {
		out = append(out, tail)
	}
	return out
}

func containsAny(sentence string, keys []string) bool {
	s := strings.ToLower(sentence)
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

func containsListaA(sentence string) bool { return containsAny(sentence, listaA) }
func containsListaB(sentence string) bool { return containsAny(sentence, listaB) }

// extractQuestionCore:
// - 1) Encontra a PRIMEIRA frase que contém Lista A e retorna essa + as seguintes.
// - 2) Se não existir Lista A, tenta Lista B (somente se tiver "?" na frase).
// - 3) Se nada, retorna "".
func extractQuestionCore(text string) string {
	sentences := splitSentences(text)

	for i, s := range sentences {
		if containsListaA(s) {
			return strings.TrimSpace(strings.Join(sentences[i:], " "))
		}
	}

	for i, s := range sentences {
		if containsListaB(s) && strings.Contains(s, "?") {
			return strings.TrimSpace(strings.Join(sentences[i:], " "))
		}
	}

	return ""
}

// =====================
// Recall cleanup
// =====================

func stripTDLoadLogs(s string) string {
	lines := strings.Split(s, "\n")
	out := []string{}

	for _, line := range lines {
		lt := strings.TrimSpace(line)
		if strings.HasPrefix(lt, "🧠 [PERSIST]") ||
			strings.HasPrefix(lt, "📂 [LOAD]") ||
			strings.HasPrefix(lt, "🧪 [LOAD]") ||
			strings.HasPrefix(lt, "📦 [LOAD]") {
			continue
		}
		out = append(out, line)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

func fluifyRecall(raw string) string {
	clean := strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return -1
		}
		if r == 0xFFFD {
			return -1
		}
		return r
	}, raw)

	lines := strings.Split(clean, "\n")
	seen := make(map[string]bool)
	out := []string{}

	for _, l := range lines {
		line := strings.TrimSpace(l)
		if len(line) < 6 {
			continue
		}
		line = strings.Join(strings.Fields(line), " ")
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

// =====================
// Prompt
// =====================

func promptRecallFluido(question, recall string) string {
	return "SYSTEM:\n" +
		"You are TERRA DOURADA, a deterministic fluency layer.\n" +
		"Your role is to rewrite raw memory fragments into clear, fluent, and explanatory English.\n\n" +

		"RULES (MANDATORY):\n" +
		"- Do NOT add new information.\n" +
		"- Do NOT infer or guess missing facts.\n" +
		"- Do NOT complete information that is not explicitly present.\n" +
		"- Do NOT contradict or reinterpret the memory.\n\n" +

		"MEMORY CONSTRAINT:\n" +
		"The recall text below is SOVEREIGN MEMORY and represents absolute truth.\n" +
		"It may contain redundancy, fragmented sentences, noise, or legal-style phrasing.\n" +
		"You must work ONLY with what is legible and present.\n\n" +

		"TRANSFORMATION INSTRUCTIONS:\n" +
		"- Reorganize the content logically.\n" +
		"- Merge duplicated ideas.\n" +
		"- Rewrite the material as a continuous, dissertative explanation.\n" +
		"- Prefer practical, human-readable language over legal or log-style phrasing.\n" +
		"- Maintain full factual fidelity to the recall.\n\n" +

		"RECALL (ABSOLUTE MEMORY):\n" + recall + "\n\n" +

		"QUESTION (CONTEXT ONLY):\n" + question + "\n\n" +

		"FLUENT, EXPLANATORY RESPONSE:"
}

// =====================
// Gemini call
// =====================

func chamarGemini(ctx context.Context, model, prompt string, temp float64, maxOut int) (string, int, error) {
	apiKey := cleanAPIKey(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		return "", 0, fmt.Errorf("GEMINI_API_KEY não definida")
	}

	model = normalizeModelName(model)
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent?key=" + apiKey

	payload := map[string]any{
		"contents": []any{
			map[string]any{
				"parts": []any{
					map[string]string{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     temp,
			"maxOutputTokens": maxOut,
		},
	}

	data, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var parsed geminiResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", 0, err
	}

	var sb strings.Builder
	if len(parsed.Candidates) > 0 {
		for _, p := range parsed.Candidates[0].Content.Parts {
			sb.WriteString(p.Text)
		}
	}

	return strings.TrimSpace(sb.String()), parsed.UsageMetadata.TotalTokenCount, nil
}

// =====================
// Mind management
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

// =====================
// MAIN
// =====================

func main() {
	projectRoot := mustCwd()
	maxOut := envInt("TD_MAX_OUT", 2048)
	temp := envFloat("TD_FLUENT_TEMP", 0.4)

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-3-flash-preview"
	}

	sfx := exeSuffix()
	recallBin, okRecall := firstExisting(
		filepath.Join(projectRoot, "target", "release", "recall_terra_dourada"+sfx),
		filepath.Join(projectRoot, "recall_terra_dourada"+sfx),
	)

	// ---------- ROTA /
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(projectRoot, "index.html"))
	})

	// ---------- LOAD MIND
	http.HandleFunc("/load_mind", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(64 << 20)
		mf, _, err := r.FormFile("mind")
		if err != nil {
			http.Error(w, "arquivo não enviado", 400)
			return
		}
		defer mf.Close()

		tmp, _ := os.CreateTemp("", "td_mind_*.bin")
		n, _ := io.Copy(tmp, mf)
		_ = tmp.Close()

		setLoadedMind(tmp.Name())
		_ = json.NewEncoder(w).Encode(LoadMindResponse{Ok: true, Size: n, Mode: "loaded"})
	})

	// ---------- ASK
	http.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
		var p struct {
			Question string `json:"question"`
		}
		_ = json.NewDecoder(r.Body).Decode(&p)

		rawQuestion := strings.TrimSpace(p.Question)
		questionCore := extractQuestionCore(rawQuestion)
		if questionCore == "" {
			// Sem núcleo detectável: mantém comportamento antigo (não quebra).
			questionCore = rawQuestion
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancel()

		recall := ""
		mode := "vazio"

		if okRecall && getLoadedMind() != "" && strings.TrimSpace(questionCore) != "" {
			var out bytes.Buffer
			// ✅ Recall recebe só a pergunta (núcleo)
			cmd := exec.CommandContext(ctx, recallBin, questionCore)
			cmd.Env = append(os.Environ(), "TD_MIND_PATH="+getLoadedMind())
			cmd.Stdout = &out
			_ = cmd.Run()

			recall = fluifyRecall(stripTDLoadLogs(out.String()))
			mode = "recall_raw"
		}

		answer := recall
		tokens := 0

		if recall != "" {
			// ✅ Gemini continua recebendo o texto inteiro (rawQuestion)
			prompt := promptRecallFluido(rawQuestion, recall)
			resp, tk, err := chamarGemini(ctx, model, prompt, temp, maxOut)
			if err == nil && resp != "" {
				answer = resp
				tokens = tk
				mode = "recall_fluido"
			}
		}

		_ = json.NewEncoder(w).Encode(AskResponse{
			Answer: answer,
			Recall: recall,
			Mode:   mode,
			Tokens: tokens,
		})
	})

	log.Printf("🚀 Server rodando em http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
