package ui

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

// Config komut satırı bayraklarından gelen çalışma ayarları.
type Config struct {
	DataDir       string
	DownloadDir   string
	UploadDir     string
	Port          int // 0 = rastgele
	DiscoveryPort int
}

var Cfg = Config{
	DataDir:       "veri",
	DownloadDir:   "indirilenler",
	UploadDir:     "gönderilecekler",
	Port:          0,
	DiscoveryPort: 8888,
}

// Settings kullanıcıya ait, diske kaydedilen tercihler.
type Settings struct {
	Username string `json:"username"`
	Theme    string `json:"theme"`
	ShowTime bool   `json:"show_time"`
}

func settingsPath() string { return filepath.Join(Cfg.DataDir, "ayarlar.json") }

func loadSettings() Settings {
	s := Settings{Theme: themes[0].name, ShowTime: true}
	if data, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(data, &s)
	}
	return s
}

func (s Settings) save() {
	os.MkdirAll(Cfg.DataDir, os.ModePerm)
	if data, err := json.MarshalIndent(s, "", "  "); err == nil {
		os.WriteFile(settingsPath(), data, 0644)
	}
}

type theme struct {
	name                       string
	primary, secondary, accent lipgloss.Color
}

var themes = []theme{
	{"Okyanus", "#00f2fe", "#4facfe", "#ff0844"},
	{"Orman", "#7ee787", "#3fb950", "#ff7b72"},
	{"Gün Batımı", "#ffb86c", "#ff79c6", "#ff5555"},
	{"Mono", "#ffffff", "#aaaaaa", "#ff5555"},
}

func themeIndex(name string) int {
	for i, t := range themes {
		if t.name == name {
			return i
		}
	}
	return 0
}
