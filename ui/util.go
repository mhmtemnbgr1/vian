package ui

import (
	"fmt"
	"hash/fnv"
	"net"
	"os"
	"strings"
	"time"
	"vian/network"

	"github.com/charmbracelet/lipgloss"
)

// localIP yerel ağdaki IP adresini döndürür (paket göndermez, sadece rota seçer).
func localIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "bilinmiyor"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}

var senderPalette = []lipgloss.Color{
	"#ffb199", "#c3e88d", "#f78c6c", "#c792ea", "#89ddff", "#ffcb6b",
}

// senderStyle her kullanıcı adı için sabit bir renk üretir.
func senderStyle(name string) lipgloss.Style {
	h := fnv.New32a()
	h.Write([]byte(name))
	return lipgloss.NewStyle().Bold(true).Foreground(senderPalette[h.Sum32()%uint32(len(senderPalette))])
}

// cleanPath /send ile gelen yoldaki tırnakları ve boşlukları temizler.
func cleanPath(p string) string {
	return strings.Trim(strings.TrimSpace(p), `"'`)
}

// randomPassword okunması kolay (karışan karakterler olmadan) rastgele bir parola üretir, örn. "k3x9m-p2qrt".
func randomPassword() string {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	b, err := network.RandomBytes(10)
	if err != nil {
		return "vian-" + fmt.Sprint(time.Now().UnixNano()%1000000)
	}
	out := make([]byte, 0, 11)
	for i, x := range b {
		if i == 5 {
			out = append(out, '-')
		}
		out = append(out, alphabet[int(x)%len(alphabet)])
	}
	return string(out)
}

const maxDropFiles = 10

// parsePaths sürükle-bırak ile (veya yapıştırılarak) gelen metni dosya yollarına çevirir.
// Tırnaklı ("C:\a b\c.txt", 'x'), boşlukla ayrılmış birden çok yolu ve tek bir boşluklu yolu destekler.
// Metindeki her parça mevcut bir dosya değilse nil döner (yani sıradan mesajdır).
func parsePaths(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "& ") // PowerShell biçimi: & 'C:\yol'
	if s == "" || len(s) > 4096 || strings.ContainsAny(s, "\n\r") && !strings.Contains(strings.TrimSpace(s), "\n") {
		return nil
	}
	isFile := func(p string) bool {
		st, err := os.Stat(p)
		return err == nil && st.Mode().IsRegular()
	}

	// Tek yol (boşluk içerse bile)
	if one := cleanPath(s); isFile(one) {
		return []string{one}
	}

	// Tırnak ve boşluk kurallarına göre parçala
	var parts []string
	var cur strings.Builder
	var quote rune
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
	}
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
				flush()
			} else {
				cur.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\n' || r == '\r' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()

	if len(parts) == 0 || len(parts) > maxDropFiles {
		return nil
	}
	for _, p := range parts {
		if !isFile(p) {
			return nil
		}
	}
	return parts
}
