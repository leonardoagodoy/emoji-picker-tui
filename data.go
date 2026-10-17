package main

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed emojis.tsv
var emojiTSV string

type emoji struct {
	char, group, pt, en string
	keys                string // minúsculo, para a busca
}

// groups: nome interno do Unicode → rótulo da aba
var groups = []struct{ id, label string }{
	{"Smileys & Emotion", "rostos"},
	{"People & Body", "pessoas"},
	{"Animals & Nature", "natureza"},
	{"Food & Drink", "comida"},
	{"Travel & Places", "viagem"},
	{"Activities", "atividades"},
	{"Objects", "objetos"},
	{"Symbols", "símbolos"},
	{"Flags", "bandeiras"},
}

func load() []emoji {
	var out []emoji
	for _, line := range strings.Split(emojiTSV, "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 5 {
			continue
		}
		out = append(out, emoji{f[0], f[1], f[2], f[3],
			strings.ToLower(f[2] + " " + f[3] + " " + f[4])})
	}
	return out
}

// ── recentes ────────────────────────────────────────────────────────────────

const maxRecent = 40

func recentPath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "emoji-picker", "recent")
}

func loadRecent() []string {
	b, err := os.ReadFile(recentPath())
	if err != nil {
		return nil
	}
	return strings.Fields(string(b))
}

func saveRecent(char string) {
	list := []string{char}
	for _, r := range loadRecent() {
		if r != char {
			list = append(list, r)
		}
	}
	list = list[:min(len(list), maxRecent)]
	p := recentPath()
	if os.MkdirAll(filepath.Dir(p), 0o755) == nil {
		os.WriteFile(p, []byte(strings.Join(list, "\n")+"\n"), 0o644)
	}
}

// ── clipboard ───────────────────────────────────────────────────────────────

func copyText(s string) error {
	var cmd *exec.Cmd
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		cmd = exec.Command("wl-copy", "--", s)
	} else {
		cmd = exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(s)
	}
	return cmd.Run()
}
