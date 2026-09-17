package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

// promptFileEvidence describes a single prompt file read from AC_PROMPT_DIR.
type promptFileEvidence struct {
	Name      string `json:"name"`
	Exists    bool   `json:"exists"`
	SizeBytes int64  `json:"size_bytes"`
	CharCount int    `json:"char_count"`
	SHA256    string `json:"sha256"`
	ReadError string `json:"read_error,omitempty"`
}

// readPromptFileEvidence reads a prompt file from dir and returns metadata.
// It never panics; errors are captured in ReadError.
func readPromptFileEvidence(dir, name string) promptFileEvidence {
	ev := promptFileEvidence{Name: name}
	if dir == "" {
		ev.ReadError = "prompt_dir_not_configured"
		return ev
	}
	path := filepath.Join(dir, name)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			ev.ReadError = "not_found"
		} else {
			ev.ReadError = err.Error()
		}
		return ev
	}
	ev.Exists = true
	ev.SizeBytes = info.Size()
	data, err := os.ReadFile(path)
	if err != nil {
		ev.ReadError = err.Error()
		return ev
	}
	text := string(data)
	ev.CharCount = len([]rune(text))
	sum := sha256.Sum256(data)
	ev.SHA256 = hex.EncodeToString(sum[:])
	return ev
}

// buildPromptAssemblyTrace gathers read-only prompt evidence from AC_PROMPT_DIR.
// It is safe to call with an empty promptDir (degrades to not_configured).
func buildPromptAssemblyTrace(promptDir string) map[string]any {
	supervisorSystem := readPromptFileEvidence(promptDir, "supervisor_system.txt")
	criticSystem := readPromptFileEvidence(promptDir, "critic_system.txt")
	supervisorPrompt := readPromptFileEvidence(promptDir, "supervisor_prompt.txt")
	criticPrompt := readPromptFileEvidence(promptDir, "critic_prompt.txt")

	files := []promptFileEvidence{supervisorSystem, criticSystem, supervisorPrompt, criticPrompt}
	existsCount := 0
	totalChars := 0
	for _, f := range files {
		if f.Exists {
			existsCount++
			totalChars += f.CharCount
		}
	}

	source := promptSourceStatus(promptDir)
	return map[string]any{
		"prompt_source":  source,
		"prompt_dir":     promptDir,
		"files":          files,
		"files_found":    existsCount,
		"files_expected": 4,
		"total_chars":    totalChars,
		"would_call_llm": false,
		"upstream_write": "disabled",
		"read_source":    "AC_PROMPT_DIR read-only prompt evidence",
	}
}
