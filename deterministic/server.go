package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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
// Limpeza de logs conhecidos
// =====================

func stripTDLoadLogs(s string) string {
	if s == "" {
		return s
	}

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))

	for _, line := range lines {
		l := strings.TrimRight(line, "\r")
		lt := strings.TrimSpace(l)

		if strings.HasPrefix(lt, "🧠 [PERSIST]") {
			continue
		}
		if strings.HasPrefix(lt, "📂 [LOAD]") {
			continue
		}
		if strings.HasPrefix(lt, "🧪 [LOAD]") {
			continue
		}
		if strings.HasPrefix(lt, "📦 [LOAD]") {
			continue
		}

		out = append(out, l)
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

// =====================
// Fluidez soberana
// NÃO inventa
// NÃO completa
// NÃO interpreta
// =====================

func fluifyRecall(raw string) string {
	if raw == "" {
		return raw
	}

	// remove lixo binário / unicode quebrado
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
	out := make([]string, 0, len(lines))

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

	text := strings.Join(out, "\n")

	text = strings.ReplaceAll(text, " .", ".")
	text = strings.ReplaceAll(text, " ,", ",")
	text = strings.ReplaceAll(text, " %", "%")

	return strings.TrimSpace(text)
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
// Multipart helpers
// =====================

func getMultipartFile(r *http.Request, field string) (multipart.File, *multipart.FileHeader, error) {
	f, fh, err := r.FormFile(field)
	if err == nil {
		return f, fh, nil
	}
	return nil, nil, err
}

func saveToTempFile(prefix string, data io.Reader) (string, int64, error) {
	tmp, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", 0, err
	}
	defer tmp.Close()

	n, err := io.Copy(tmp, data)
	if err != nil {
		return "", 0, err
	}
	return tmp.Name(), n, nil
}

// =====================
// Gestão de Mind
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
// MAIN
// =====================

func main() {
	projectRoot := os.Getenv("TD_PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot = mustCwd()
	}

	maxOut := envInt("TD_MAX_OUT", 2048)

	sfx := exeSuffix()
	recallBin, okRecall := firstExisting(
		filepath.Join(projectRoot, "target", "release", "recall_terra_dourada"+sfx),
		filepath.Join(projectRoot, "recall_terra_dourada"+sfx),
	)

	log.Printf("🚀 Servidor Rodando na Porta :8080")
	log.Printf("📦 TD_MAX_OUT: %d", maxOut)
	if okRecall {
		log.Printf("🧠 Recall bin: %s", recallBin)
	} else {
		log.Printf("⚠️ Recall bin NÃO encontrado")
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}
		http.ServeFile(w, r, filepath.Join(projectRoot, "index.html"))
	})

	http.HandleFunc("/load_mind", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			http.Error(w, "multipart inválido", 400)
			return
		}

		mf, _, err := getMultipartFile(r, "mind")
		if err != nil {
			http.Error(w, "Arquivo não enviado", 400)
			return
		}
		defer mf.Close()

		mindTemp, n, err := saveToTempFile("td_mind_*.bin", mf)
		if err != nil {
			http.Error(w, "Falha salvando mind", 500)
			return
		}
		setLoadedMind(mindTemp)

		_ = json.NewEncoder(w).Encode(LoadMindResponse{
			Ok:   true,
			Size: n,
			Mode: "loaded",
		})
	})

	// =====================
	// /ask — recall soberano + fluidez
	// =====================

	http.HandleFunc("/ask", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 2<<20)

		var p struct {
			Question string `json:"question"`
		}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}

		q := strings.TrimSpace(p.Question)
		if q == "" {
			http.Error(w, "question vazia", 400)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancel()

		mp := getLoadedMind()

		recallRaw := ""
		mode := "vazio"

		if okRecall && mp != "" {
			var out bytes.Buffer
			var errb bytes.Buffer

			cmd := exec.CommandContext(ctx, recallBin, q)
			cmd.Env = append(os.Environ(), "TD_MIND_PATH="+mp)
			cmd.Stdout = &out
			cmd.Stderr = &errb

			if err := cmd.Run(); err != nil {
				log.Printf("❌ recall erro: %v | stderr=%s", err, errb.String())
			}

			recallRaw = strings.TrimSpace(out.String())
			recallRaw = stripTDLoadLogs(recallRaw)
			recallRaw = fluifyRecall(recallRaw)

			mode = "recall_raw"
			if recallRaw == "" {
				mode = "recall_vazio"
			}
		} else {
			if !okRecall {
				mode = "sem_bin"
			} else if mp == "" {
				mode = "sem_mind"
			}
		}

		_ = json.NewEncoder(w).Encode(AskResponse{
			Answer: recallRaw,
			Recall: recallRaw,
			Mode:   mode,
			Tokens: 0,
		})
	})

	http.HandleFunc("/unload_mind", func(w http.ResponseWriter, r *http.Request) {
		if cors(w, r) {
			return
		}
		clearLoadedMind()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":   true,
			"mode": "cleared",
		})
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}  
