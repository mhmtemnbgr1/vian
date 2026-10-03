package transfer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"foto.png":         "foto.png",
		"../../etc/passwd": "passwd",
		`..\..\win.ini`:    "win.ini",
		"..":               "dosya",
		"":                 "dosya",
	}
	for in, want := range cases {
		if got := SafeName(in); got != want {
			t.Errorf("SafeName(%q) = %q, beklenen %q", in, got, want)
		}
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	if got := UniquePath(dir, "a.txt"); got != filepath.Join(dir, "a.txt") {
		t.Errorf("got %q", got)
	}
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644)
	if got := UniquePath(dir, "a.txt"); got != filepath.Join(dir, "a (1).txt") {
		t.Errorf("got %q", got)
	}
}

func TestSHA256File(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x")
	os.WriteFile(p, []byte("abc"), 0644)
	got, err := SHA256File(p)
	if err != nil || got != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("got %q err %v", got, err)
	}
}
