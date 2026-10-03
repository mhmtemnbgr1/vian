package ui

// README görselleri: uygulamanın gerçek ekran çıktısını (ANSI renkleriyle) SVG'ye çevirir.
// Yeniden üretmek için:  VIAN_SHOTS=1 go test ./ui -run TestGenerateScreenshots

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"vian/network"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

type cellStyle struct {
	fg, bg           string
	bold, italic, ul bool
	faint, reverse   bool
}

var sgrRe = regexp.MustCompile("\x1b\\[([0-9;]*)m")
var csiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]")

var ansi16 = []string{
	"#000000", "#cd3131", "#0dbc79", "#e5e510", "#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5",
	"#666666", "#f14c4c", "#23d18b", "#f5f543", "#3b8eea", "#d670d6", "#29b8db", "#ffffff",
}

func color256(n int) string {
	switch {
	case n < 16:
		return ansi16[n]
	case n < 232:
		n -= 16
		c := func(v int) int {
			if v == 0 {
				return 0
			}
			return 55 + v*40
		}
		return fmt.Sprintf("#%02x%02x%02x", c(n/36), c(n/6%6), c(n%6))
	default:
		g := 8 + (n-232)*10
		return fmt.Sprintf("#%02x%02x%02x", g, g, g)
	}
}

func applySGR(st cellStyle, params string) cellStyle {
	if params == "" {
		return cellStyle{}
	}
	p := strings.Split(params, ";")
	num := func(i int) int { v, _ := strconv.Atoi(p[i]); return v }
	for i := 0; i < len(p); i++ {
		n := num(i)
		switch {
		case n == 0:
			st = cellStyle{}
		case n == 1:
			st.bold = true
		case n == 2:
			st.faint = true
		case n == 3:
			st.italic = true
		case n == 4:
			st.ul = true
		case n == 7:
			st.reverse = true
		case n == 22:
			st.bold, st.faint = false, false
		case n == 23:
			st.italic = false
		case n == 24:
			st.ul = false
		case n == 27:
			st.reverse = false
		case n >= 30 && n <= 37:
			st.fg = ansi16[n-30]
		case n >= 90 && n <= 97:
			st.fg = ansi16[n-90+8]
		case n >= 40 && n <= 47:
			st.bg = ansi16[n-40]
		case n >= 100 && n <= 107:
			st.bg = ansi16[n-100+8]
		case n == 39:
			st.fg = ""
		case n == 49:
			st.bg = ""
		case n == 38 || n == 48:
			target := &st.fg
			if n == 48 {
				target = &st.bg
			}
			if i+4 < len(p)+0 && num(i+1) == 2 && i+4 <= len(p)-1 {
				*target = fmt.Sprintf("#%02x%02x%02x", num(i+2), num(i+3), num(i+4))
				i += 4
			} else if i+2 <= len(p)-1 && num(i+1) == 5 {
				*target = color256(num(i + 2))
				i += 2
			}
		}
	}
	return st
}

type cell struct {
	r     rune
	w     int
	style cellStyle
}

func parseANSI(s string) [][]cell {
	var rows [][]cell
	for _, line := range strings.Split(s, "\n") {
		var row []cell
		st := cellStyle{}
		rest := line
		for len(rest) > 0 {
			if loc := sgrRe.FindStringSubmatchIndex(rest); loc != nil && loc[0] == 0 {
				st = applySGR(st, rest[loc[2]:loc[3]])
				rest = rest[loc[1]:]
				continue
			}
			if loc := csiRe.FindStringIndex(rest); loc != nil && loc[0] == 0 {
				rest = rest[loc[1]:]
				continue
			}
			r := []rune(rest)[0]
			rest = rest[len(string(r)):]
			w := runewidth.RuneWidth(r)
			if w == 0 {
				continue
			}
			row = append(row, cell{r: r, w: w, style: st})
		}
		rows = append(rows, row)
	}
	return rows
}

func toSVG(view, title string, cols, rows int) string {
	const (
		cw, ch   = 9.0, 19.0
		pad      = 18.0
		titleBar = 38.0
		defFg    = "#d7dae0"
		defBg    = "#0d1117"
	)
	grid := parseANSI(view)
	W := cols*int(cw) + int(pad)*2
	H := rows*int(ch) + int(pad)*2 + int(titleBar)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="'Cascadia Mono','Cascadia Code',Consolas,Menlo,'DejaVu Sans Mono',monospace" font-size="15">`+"\n", W, H, W, H)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" rx="10" fill="%s" stroke="#30363d"/>`+"\n", W, H, defBg)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" rx="10" fill="#161b22"/><rect y="20" width="%d" height="%d" fill="#161b22"/>`+"\n", W, int(titleBar), W, int(titleBar)-20)
	for i, c := range []string{"#ff5f56", "#ffbd2e", "#27c93f"} {
		fmt.Fprintf(&b, `<circle cx="%d" cy="19" r="6" fill="%s"/>`, 20+i*20, c)
	}
	fmt.Fprintf(&b, `<text x="%d" y="24" text-anchor="middle" fill="#8b949e" font-size="13">%s</text>`+"\n", W/2, html.EscapeString(title))

	for ri := 0; ri < rows && ri < len(grid); ri++ {
		y := titleBar + pad + float64(ri)*ch
		row := grid[ri]
		// aynı stildeki ardışık hücreleri tek çalıştırmada birleştir
		col := 0
		for i := 0; i < len(row); {
			j, width := i, 0
			var txt strings.Builder
			for j < len(row) && row[j].style == row[i].style {
				txt.WriteRune(row[j].r)
				width += row[j].w
				j++
			}
			st := row[i].style
			fg, bg := st.fg, st.bg
			if st.reverse {
				fg, bg = bg, fg
				if fg == "" {
					fg = defBg
				}
				if bg == "" {
					bg = defFg
				}
			}
			if fg == "" {
				fg = defFg
			}
			x := pad + float64(col)*cw
			if bg != "" {
				fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`, x, y, float64(width)*cw, ch, bg)
			}
			if t := txt.String(); strings.TrimSpace(t) != "" {
				attrs := ""
				if st.bold {
					attrs += ` font-weight="bold"`
				}
				if st.italic {
					attrs += ` font-style="italic"`
				}
				if st.ul {
					attrs += ` text-decoration="underline"`
				}
				if st.faint {
					attrs += ` opacity="0.6"`
				}
				fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s"%s xml:space="preserve" textLength="%.1f" lengthAdjust="spacingAndGlyphs">%s</text>`,
					x, y+ch-5, fg, attrs, float64(width)*cw, html.EscapeString(t))
			}
			col += width
			i = j
		}
		b.WriteString("\n")
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func TestGenerateScreenshots(t *testing.T) {
	if os.Getenv("VIAN_SHOTS") == "" {
		t.Skip("VIAN_SHOTS=1 ile çalıştırın")
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer applyTheme(0)

	outDir := filepath.Join("..", "docs", "screenshots")
	os.MkdirAll(outDir, 0755)

	const cols, rows = 110, 32
	base := func() model {
		m := testModel(t)
		// Görsellerde gerçek/geçici yollar yerine varsayılan klasör adları görünsün.
		Cfg.DownloadDir, Cfg.UploadDir = "indirilenler", "gönderilecekler"
		m.username = "Memo"
		m.localAddr = "192.168.1.20"
		m.now = time.Date(2026, 1, 15, 12, 30, 0, 0, time.Local)
		m.width, m.height = cols, rows
		m.settings.ShowTime = true
		return m
	}
	chatLines := func(m *model) {
		m.lines = []chatLine{
			{text: "Canlı sohbet", kind: lineDivider},
			{at: "12:28", text: "'Gizli Karargah' odası hazır. Adres: 192.168.1.20:52711", kind: lineSystem},
			{at: "12:29", text: "Ali odaya katıldı. (Odada 2 kişi)", kind: lineSystem},
			{at: "12:29", sender: "Ali", text: "Selam! Rapor hazır, birazdan gönderiyorum.", kind: lineChat},
			{at: "12:30", sender: "Memo", text: "Süper, bekliyorum. Parolayı da odanın adından türettim :)", kind: lineChat},
			{at: "12:30", text: "Ayşe odaya katıldı. (Odada 3 kişi)", kind: lineSystem},
			{at: "12:31", sender: "Ayşe", text: "Merhaba! Dosyayı ben de alabilir miyim? Toplantı notlarını da ekleyebilirim.", kind: lineChat},
		}
		m.refreshViewport()
	}

	shots := []struct {
		file, title string
		build       func() model
	}{
		{"01-giris.svg", "vian · hoş geldiniz", func() model {
			m := base()
			m.state = stateUserSetup
			m.usernameInput.SetValue("Memo")
			return m
		}},
		{"02-ana-ekran.svg", "vian · ana ekran", func() model {
			m := base()
			m.state = stateHome
			m.joinStop = make(chan struct{})
			now := time.Now()
			m.foundPeers = []foundPeer{
				{peerFoundMsg{ip: "192.168.1.34", msg: network.DiscoveryMessage{Port: 52711, HostName: "Gizli Karargah", Owner: "Ali"}}, now},
				{peerFoundMsg{ip: "192.168.1.41", msg: network.DiscoveryMessage{Port: 49820, HostName: "Proje Toplantısı", Owner: "Ayşe"}}, now},
			}
			m.focus = 4
			return m
		}},
		{"03-oda-kur.svg", "vian · oda kur", func() model {
			m := base()
			m.state = stateHostForm
			m.hostNameInput.SetValue("Gizli Karargah")
			m.hostInput.SetValue("k3x9m-p2qrt")
			m.showPass = true
			m.applyEcho()
			m.formFocus = 2
			return m
		}},
		{"04-odaya-katil.svg", "vian · odaya katıl", func() model {
			m := base()
			m.state = stateJoinForm
			m.joiningPeer = &peerFoundMsg{ip: "192.168.1.34", msg: network.DiscoveryMessage{Port: 52711, HostName: "Gizli Karargah", Owner: "Ali"}}
			m.joinInput.SetValue("parola123")
			m.joinInput.Focus()
			return m
		}},
		{"05-sohbet.svg", "vian · sohbet", func() model {
			m := base()
			m.roomName, m.isHosting, m.hostIP, m.hostPort = "Gizli Karargah", true, "192.168.1.20", 52711
			m.nodes = []*network.Node{{Name: "Ali"}, {Name: "Ayşe"}}
			m.enterChat()
			chatLines(&m)
			m.typing["Ali"] = time.Now()
			return m
		}},
		{"06-dosya-aktarimi.svg", "vian · dosya aktarımı", func() model {
			m := base()
			m.roomName, m.isHosting, m.hostIP, m.hostPort = "Gizli Karargah", true, "192.168.1.20", 52711
			m.nodes = []*network.Node{{Name: "Ali"}, {Name: "Ayşe"}}
			m.enterChat()
			chatLines(&m)
			m.lines = append(m.lines,
				chatLine{at: "12:31", text: "Ali bir dosya göndermek istiyor: rapor.pdf", kind: lineSystem})
			m.offers = []incomingOffer{{offer: network.FileOffer{FileID: "x", FileName: "rapor.pdf", FileSize: 4_404_019}, node: &network.Node{Name: "Ali"}}}
			m.refreshViewport()
			return m
		}},
		{"07-ayarlar.svg", "vian · ayarlar", func() model {
			m := base()
			m.state = stateSettings
			m.settingsCursor = 1
			return m
		}},
		{"08-yardim.svg", "vian · yardım", func() model {
			m := base()
			m.state = stateHelp
			return m
		}},
	}

	for _, s := range shots {
		m := s.build()
		svg := toSVG(m.View(), s.title, cols, rows)
		if err := os.WriteFile(filepath.Join(outDir, s.file), []byte(svg), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Tema galerisi: ana ekranı dört temayla
	for i, th := range themes {
		applyTheme(i)
		m := base()
		m.state = stateHome
		m.joinStop = make(chan struct{})
		m.settings.Theme = th.name
		m.foundPeers = []foundPeer{{peerFoundMsg{ip: "192.168.1.34", msg: network.DiscoveryMessage{Port: 52711, HostName: "Gizli Karargah", Owner: "Ali"}}, time.Now()}}
		m.focus = 4
		name := fmt.Sprintf("tema-%d.svg", i+1)
		os.WriteFile(filepath.Join(outDir, name), []byte(toSVG(m.View(), "tema: "+th.name, cols, rows)), 0644)
	}
}
