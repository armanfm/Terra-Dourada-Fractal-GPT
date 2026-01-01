package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// =====================
// DTOs
// =====================

type AskRequest struct {
	Question string `json:"question"`
}

type AskResponse struct {
	Recall       string `json:"recall"`
	QuestionCore string `json:"question_core"`
	Mode         string `json:"mode"`
}

type LoadMindResponse struct {
	Ok   bool  `json:"ok"`
	Size int64 `json:"size"`
	Mode string `json:"mode"`
}

// =====================
// Helpers
// =====================

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
	// PT
	"posso",
	"consigo",
	"poderia",
	"dá pra",
	"da pra",
	"é possível",
	"e possivel",

	// EN
	"can i",
	"could i",
	"can you",
	"could you",
	"would you",
	"am i able to",
	"is it possible",
}

var listaBSorted = func() []string {
	cp := append([]string{}, listaB...)
	sort.SliceStable(cp, func(i, j int) bool { return len(cp[i]) > len(cp[j]) })
	return cp
}()

func splitSentences(text string) []string {
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

// =====================
// Lista B merge soberano (A só como fonte; SOMENTE B vai pro recall)
// =====================

func normalizeToken(tok string) string {
	tok = strings.ToLower(tok)
	tok = strings.TrimSpace(tok)
	tok = strings.Trim(tok, " \t\n\r.,!?;:\"'`´()[]{}<>|/\\")
	tok = strings.ReplaceAll(tok, "'", "")
	tok = strings.ReplaceAll(tok, "’", "")
	tok = strings.ReplaceAll(tok, "´", "")
	return tok
}

func tokensOrderedUnique(s string) []string {
	fields := strings.Fields(s)
	seen := make(map[string]bool)
	out := []string{}
	for _, f := range fields {
		t := normalizeToken(f)
		if t == "" || len(t) < 2 {
			continue
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func diffTokens(prev, curr string) string {
	a := tokensOrderedUnique(prev)
	b := tokensOrderedUnique(curr)

	setB := make(map[string]bool, len(b))
	for _, t := range b {
		setB[t] = true
	}

	out := []string{}
	for _, t := range a {
		if !setB[t] {
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

func findListaBMatch(sentence string) (int, string) {
	lower := strings.ToLower(sentence)

	bestIdx := -1
	bestKey := ""

	for _, k := range listaBSorted {
		if k == "" {
			continue
		}
		i := strings.Index(lower, k)
		if i < 0 {
			continue
		}
		if bestIdx == -1 || i < bestIdx || (i == bestIdx && len(k) > len(bestKey)) {
			bestIdx = i
			bestKey = k
		}
	}
	return bestIdx, bestKey
}

func rebuildListaB(prevA, currB string) string {
	currB = strings.TrimSpace(currB)
	d := diffTokens(prevA, currB)
	if d == "" {
		return currB
	}

	idx, key := findListaBMatch(currB)
	if idx < 0 || key == "" {
		if strings.Contains(currB, "?") {
			return strings.TrimSpace(currB + " " + d)
		}
		return strings.TrimSpace(currB + " " + d + "?")
	}

	insertPos := idx + len(key)
	if insertPos < 0 || insertPos > len(currB) {
		if strings.Contains(currB, "?") {
			return strings.TrimSpace(currB + " " + d)
		}
		return strings.TrimSpace(currB + " " + d + "?")
	}

	prefix := strings.TrimSpace(currB[:insertPos])
	rest := strings.TrimSpace(currB[insertPos:])

	if rest != "" {
		return strings.TrimSpace(prefix + " " + d + " " + rest)
	}

	if strings.Contains(currB, "?") {
		return strings.TrimSpace(prefix + " " + d)
	}
	return strings.TrimSpace(prefix + " " + d + "?")
}

func extractQuestionCore(text string) string {
	sentences := splitSentences(text)

	for i, s := range sentences {
		if containsListaA(s) {
			return strings.TrimSpace(strings.Join(sentences[i:], " "))
		}
	}

	for i, s := range sentences {
		if containsListaB(s) && strings.Contains(s, "?") {
			if i > 0 {
				return rebuildListaB(sentences[i-1], s)
			}
			return strings.TrimSpace(s)
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
		if strings.HasPrefix(lt, "[PERSIST]") ||
			strings.HasPrefix(lt, "[LOAD]") ||
			strings.HasPrefix(lt, "PERSIST") ||
			strings.HasPrefix(lt, "LOAD") {
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

	sfx := exeSuffix()
	recallBin, okRecall := firstExisting(
		filepath.Join(projectRoot, "target", "release", "recall_terra_dourada"+sfx),
		filepath.Join(projectRoot, "recall_terra_dourada"+sfx),
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(projectRoot, "index.html"))
	})

	http.HandleFunc("/load_mind", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		_ = r.ParseMultipartForm(64 << 20)
		mf, _, err := r.FormFile("mind")
		if err != nil {
			http.Error(w, "arquivo nao enviado", 400)
			return
		}
		defer mf.Close()

		tmp, err := os.CreateTemp("", "td_mind_*.bin")
		if err != nil {
			http.Error(w, "falha ao criar temp", 500)
			return
		}
		n, _ := io.Copy(tmp, mf)
		_ = tmp.Close()

		setLoadedMind(tmp.Name())

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(LoadMindResponse{Ok: true, Size: n, Mode: "loaded"})
	})

	http.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var p AskRequest
		_ = json.NewDecoder(r.Body).Decode(&p)

		rawQuestion := strings.TrimSpace(p.Question)
		questionCore := extractQuestionCore(rawQuestion)
		if questionCore == "" {
			questionCore = rawQuestion
		}

		mode := "vazio"
		recall := ""

		if rawQuestion == "" {
			mode = "empty_question"
		} else if !okRecall {
			mode = "recall_bin_missing"
		} else if strings.TrimSpace(getLoadedMind()) == "" {
			mode = "mind_not_loaded"
		} else if strings.TrimSpace(questionCore) == "" {
			mode = "empty_core"
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
			defer cancel()

			var out bytes.Buffer
			cmd := exec.CommandContext(ctx, recallBin, questionCore)
			cmd.Env = append(os.Environ(), "TD_MIND_PATH="+getLoadedMind())
			cmd.Stdout = &out
			_ = cmd.Run()

			recall = fluifyRecall(stripTDLoadLogs(out.String()))
			if recall == "" {
				mode = "recall_empty"
			} else {
				mode = "recall_raw"
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AskResponse{
			Recall:       recall,
			QuestionCore: questionCore,
			Mode:         mode,
		})
	})

	log.Printf("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
