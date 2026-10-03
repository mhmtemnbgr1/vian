package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatSize(t *testing.T) {
	cases := map[int64]string{0: "0 B", 1023: "1023 B", 1024: "1.0 KB", 5 << 20: "5.0 MB"}
	for in, want := range cases {
		if got := formatSize(in); got != want {
			t.Errorf("formatSize(%d) = %q, beklenen %q", in, got, want)
		}
	}
}

func TestCleanPath(t *testing.T) {
	if got := cleanPath(` "C:\a b\c.txt" `); got != `C:\a b\c.txt` {
		t.Errorf("got %q", got)
	}
}

func TestParsePaths(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte("x"), 0644)
		return p
	}
	a, b, spaced := mk("a.txt"), mk("b.png"), mk("boşluklu dosya.pdf")

	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"tek yol", a, []string{a}},
		{"tırnaklı", `"` + a + `"`, []string{a}},
		{"boşluklu yol tırnaksız", spaced, []string{spaced}},
		{"boşluklu yol tırnaklı", `"` + spaced + `"`, []string{spaced}},
		{"iki dosya", `"` + a + `" '` + b + `'`, []string{a, b}},
		{"powershell biçimi", `& '` + a + `'`, []string{a}},
		{"sıradan mesaj", "merhaba nasılsın", nil},
		{"olmayan dosya", filepath.Join(dir, "yok.txt"), nil},
		{"klasör", dir, nil},
		{"biri eksik", `"` + a + `" "` + filepath.Join(dir, "yok") + `"`, nil},
		{"boş", "  ", nil},
	}
	for _, c := range cases {
		got := parsePaths(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			}
		}
	}
}
