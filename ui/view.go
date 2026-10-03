package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// Styles (tema değişince applyTheme ile yeniden kurulur)
var (
	primaryColor   lipgloss.Color
	secondaryColor lipgloss.Color
	accentColor    lipgloss.Color
	successColor   = lipgloss.Color("#7ee787")
	mutedColor     = lipgloss.Color("#8a8f9c")

	brandStyle, panelStyle, selectedRowStyle                  lipgloss.Style
	msgUserStyle, msgSystemStyle, infoTextStyle, timeStyle    lipgloss.Style
	errorStyle, okStyle, keyStyle, titleTextStyle             lipgloss.Style
	btnStyle, btnHoverStyle, btnPrimaryStyle, btnPrimaryHover lipgloss.Style
	btnDangerStyle, btnDangerHover                            lipgloss.Style
	modalStyle                                                lipgloss.Style
)

func init() { applyTheme(0) }

func applyTheme(i int) {
	t := themes[i]
	primaryColor, secondaryColor, accentColor = t.primary, t.secondary, t.accent
	black := lipgloss.Color("#000000")

	brandStyle = lipgloss.NewStyle().Bold(true).Foreground(black).Background(primaryColor).Padding(0, 1)
	panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(secondaryColor).Padding(0, 1)
	modalStyle = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(accentColor).Padding(0, 2)
	selectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#262c3d"))

	btnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#d7dae0")).Background(lipgloss.Color("#2b3040")).Padding(0, 2)
	btnHoverStyle = btnStyle.Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#46506d")).Bold(true)
	btnPrimaryStyle = lipgloss.NewStyle().Bold(true).Foreground(black).Background(primaryColor).Padding(0, 2)
	btnPrimaryHover = btnPrimaryStyle.Background(secondaryColor).Underline(true)
	btnDangerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd7dc")).Background(lipgloss.Color("#5a2530")).Padding(0, 2)
	btnDangerHover = btnDangerStyle.Foreground(lipgloss.Color("#ffffff")).Background(accentColor).Bold(true)

	titleTextStyle = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	msgUserStyle = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	msgSystemStyle = lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
	infoTextStyle = lipgloss.NewStyle().Foreground(mutedColor)
	timeStyle = lipgloss.NewStyle().Foreground(mutedColor)
	errorStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	okStyle = lipgloss.NewStyle().Foreground(successColor).Bold(true)
	keyStyle = lipgloss.NewStyle().Foreground(secondaryColor).Bold(true)
}

// hint "tuş açıklama" biçiminde alt bilgi satırı üretir.
func hint(pairs ...string) string {
	parts := make([]string, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, keyStyle.Render(pairs[i])+" "+infoTextStyle.Render(pairs[i+1]))
	}
	return strings.Join(parts, infoTextStyle.Render("  ·  "))
}

// ---- butonlar ----

type btnKind int

const (
	btnNormal btnKind = iota
	btnPrimary
	btnDanger
)

// focusedID klavye odağındaki butonu döndürür (yoksa boş).
func (m model) focusedID() string {
	switch m.state {
	case stateHome:
		ids := homeIDs(&m)
		if m.focus >= 0 && m.focus < len(ids) {
			return ids[m.focus]
		}
	case stateHostForm:
		switch m.formFocus {
		case 2:
			return idHostCreate
		case 3:
			return idHostCancel
		}
	case stateJoinForm:
		switch m.formFocus {
		case 1:
			return idJoinGo
		case 2:
			return idJoinCancel
		}
	case stateSettings:
		switch m.settingsCursor {
		case 0:
			return idSetName
		case 2:
			return idSetTime
		case 3:
			return idSetBack
		}
	}
	return ""
}

func (m model) active(id string) bool {
	return m.hover == id || m.focusedID() == id
}

// btn tıklanabilir bir buton üretir; fare üstündeyken veya odaktayken vurgulanır.
func (m model) btn(id, label string, kind btnKind) string {
	st := btnStyle
	on := m.active(id)
	switch kind {
	case btnPrimary:
		st = btnPrimaryStyle
		if on {
			st = btnPrimaryHover
		}
	case btnDanger:
		st = btnDangerStyle
		if on {
			st = btnDangerHover
		}
	default:
		if on {
			st = btnHoverStyle
		}
	}
	return zone.Mark(id, st.Render(label))
}

// row bir ızgarada butonları aralarında boşlukla yan yana dizer.
func row(parts ...string) string {
	spaced := make([]string, 0, len(parts)*2)
	for i, p := range parts {
		if i > 0 {
			spaced = append(spaced, " ")
		}
		spaced = append(spaced, p)
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, spaced...)
}

// inputBox giriş alanını odak durumuna göre renklenen bir çerçeveye alır.
func inputBox(focused bool, view string, w int) string {
	c := mutedColor
	if focused {
		c = primaryColor
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c).Padding(0, 1).Width(w).Render(view)
}

func hex(c lipgloss.Color) (r, g, b int) {
	s := strings.TrimPrefix(string(c), "#")
	v, _ := strconv.ParseUint(s, 16, 32)
	return int(v >> 16 & 0xff), int(v >> 8 & 0xff), int(v & 0xff)
}

// gradient iki renk arasında t (0..1) noktasındaki rengi verir.
func gradient(a, b lipgloss.Color, t float64) lipgloss.Color {
	ar, ag, ab := hex(a)
	br, bg, bb := hex(b)
	l := func(x, y int) int { return x + int(float64(y-x)*t) }
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", l(ar, br), l(ag, bg), l(ab, bb)))
}

func (m model) logo() string {
	lines := []string{
		"██╗   ██╗██╗ █████╗ ███╗   ██╗",
		"██║   ██║██║██╔══██╗████╗  ██║",
		"██║   ██║██║███████║██╔██╗ ██║",
		"╚██╗ ██╔╝██║██╔══██║██║╚██╗██║",
		" ╚████╔╝ ██║██║  ██║██║ ╚████║",
		"  ╚═══╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝",
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		c := gradient(primaryColor, secondaryColor, float64(i)/float64(len(lines)-1))
		out[i] = lipgloss.NewStyle().Bold(true).Foreground(c).Render(l)
	}
	return strings.Join(out, "\n")
}

// ---- çerçeve ----

func (m model) topBar(left, right string) string {
	w := m.contentWidth()
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m model) userChip() string {
	return okStyle.Render("●") + " " + lipgloss.NewStyle().Bold(true).Render(m.username) +
		infoTextStyle.Render("  "+m.localAddr+"  "+m.now.Format("15:04"))
}

func (m model) toastLine() string {
	if m.toast == "" {
		return ""
	}
	if m.toastErr {
		return errorStyle.Render("✕ " + m.toast)
	}
	return okStyle.Render("✓ " + m.toast)
}

// frame üst çubuk + gövde + durum satırı + ipucu satırını ekrana yerleştirir.
func (m model) frame(top, body, status, hintLine string, fillBody bool) string {
	if m.width == 0 || m.height == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, top, body, status, hintLine)
	}
	w := m.contentWidth()
	if fillBody {
		h := m.height - lipgloss.Height(top) - 3
		if h < 1 {
			h = 1
		}
		body = lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, body)
	}
	parts := []string{}
	if top != "" {
		parts = append(parts, top)
	}
	parts = append(parts, body, status, hintLine)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (m model) View() string {
	var out string
	switch m.state {
	case stateUserSetup:
		out = m.viewSetup()
	case stateHome:
		out = m.viewHome()
	case stateHostForm:
		out = m.viewHostForm()
	case stateJoinForm:
		out = m.viewJoinForm()
	case stateSettings:
		out = m.viewSettings()
	case stateHelp:
		out = m.viewHelp()
	case stateChat:
		out = m.viewChat()
	case stateFilePicker:
		out = m.viewPicker()
	}
	return zone.Scan(out)
}

// ---- ekranlar ----

func (m model) viewSetup() string {
	chip := func(s string) string {
		return lipgloss.NewStyle().Foreground(primaryColor).Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).Padding(0, 1).Render(s)
	}
	body := lipgloss.JoinVertical(lipgloss.Center,
		m.logo(),
		"",
		infoTextStyle.Render("Ağ içi · parola korumalı · uçtan uca şifreli sohbet ve dosya aktarımı"),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, chip("AES-256-GCM"), " ", chip("Otomatik oda keşfi"), " ", chip("Dosya aktarımı")),
		"",
		"",
		titleTextStyle.Render("Kendine bir kullanıcı adı seç"),
		inputBox(true, m.usernameInput.View(), 40),
		"",
		m.btn(idSetupGo, "Başla  →", btnPrimary),
	)
	return m.frame("", body, "", lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, hint("Enter", "başla", "Ctrl+C", "çıkış")), true)
}

func (m model) viewHome() string {
	w := m.contentWidth()
	top := m.topBar(brandStyle.Render("VIAN")+" "+infoTextStyle.Render("Ağ içi güvenli sohbet"), m.userChip())

	toolbar := row(
		m.btn(idHomeHost, "+ Oda Kur", btnPrimary),
		m.btn(idHomeSet, "Ayarlar", btnNormal),
		m.btn(idHomeHelp, "Yardım", btnNormal),
		m.btn(idHomeQuit, "Çıkış", btnDanger),
	)

	var head string
	if m.scanErr != "" {
		head = errorStyle.Render(m.scanErr)
	} else {
		head = titleTextStyle.Render("Ağdaki odalar") + "  " + m.spin.View() +
			infoTextStyle.Render(fmt.Sprintf(" canlı tarama · %d oda", len(m.foundPeers)))
	}

	innerW := w - 4
	rows := []string{head, ""}
	if len(m.foundPeers) == 0 {
		rows = append(rows,
			infoTextStyle.Render("Henüz oda bulunamadı."),
			infoTextStyle.Render("Aynı ağdaki biri oda kurunca burada kendiliğinden belirir,"),
			infoTextStyle.Render("ya da yukarıdan kendi odanı kur."),
		)
	}
	for i, p := range m.foundPeers {
		id := idHomeJoin + strconv.Itoa(i)
		name := lipgloss.NewStyle().Bold(true).Render(p.msg.HostName)
		meta := infoTextStyle.Render(fmt.Sprintf("%s · %s:%d", p.msg.Owner, p.ip, p.msg.Port))
		marker, action := "  ", infoTextStyle.Render("Katıl ›")
		line := marker + name + "  " + meta
		if m.active(id) {
			marker, action = keyStyle.Render("▸ "), keyStyle.Render("Katıl ›")
			line = marker + name + "  " + meta
		}
		gap := innerW - lipgloss.Width(line) - lipgloss.Width(action) - 1
		if gap < 1 {
			gap = 1
		}
		full := line + strings.Repeat(" ", gap) + action
		if m.active(id) {
			full = selectedRowStyle.Render(full)
		}
		rows = append(rows, zone.Mark(id, full))
	}
	for len(rows) < 10 {
		rows = append(rows, "")
	}
	list := panelStyle.Width(innerW).Render(strings.Join(rows, "\n"))

	body := lipgloss.JoinVertical(lipgloss.Left, toolbar, "", list)
	return m.frame(top, lipgloss.PlaceHorizontal(w, lipgloss.Center, body), m.toastLine(),
		hint("↑↓", "seç", "Enter", "katıl", "N", "oda kur", "S", "ayarlar", "?", "yardım", "Q", "çıkış"), true)
}

func (m model) modal(title string, lines ...string) string {
	body := titleTextStyle.Render(title) + "\n\n" + strings.Join(lines, "\n")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(primaryColor).Padding(1, 3).Render(body)
}

func (m model) passEye(id string) string {
	label := "Göster"
	if m.showPass {
		label = "Gizle"
	}
	return m.btn(id, label, btnNormal)
}

func (m model) viewHostForm() string {
	const w = 44
	lines := []string{
		infoTextStyle.Render("Oda adı"),
		inputBox(m.formFocus == 0, m.hostNameInput.View(), w),
		"",
		infoTextStyle.Render("Parola"),
		inputBox(m.formFocus == 1, m.hostInput.View(), w),
		row(m.passEye(idHostEye), m.btn(idHostRand, "Rastgele parola", btnNormal)),
		"",
	}
	switch {
	case m.connecting:
		lines = append(lines, m.spin.View()+infoTextStyle.Render(" Oda hazırlanıyor..."), "")
	case m.errText != "":
		lines = append(lines, errorStyle.Render("✕ "+m.errText), "")
	}
	lines = append(lines, row(m.btn(idHostCreate, "Oda Kur", btnPrimary), m.btn(idHostCancel, "İptal", btnNormal)))
	body := m.modal("Yeni oda kur", lines...)
	return m.frame("", body, m.toastLine(),
		lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, hint("Tab", "alan değiştir", "Enter", "ileri / kur", "Ctrl+R", "rastgele", "Ctrl+E", "göster", "Esc", "iptal")), true)
}

func (m model) viewJoinForm() string {
	const w = 44
	name, meta := "", ""
	if m.joiningPeer != nil {
		name = m.joiningPeer.msg.HostName
		meta = fmt.Sprintf("%s · %s:%d", m.joiningPeer.msg.Owner, m.joiningPeer.ip, m.joiningPeer.msg.Port)
	}
	lines := []string{
		lipgloss.NewStyle().Bold(true).Render(name) + "  " + infoTextStyle.Render(meta),
		"",
		infoTextStyle.Render("Oda parolası"),
		inputBox(m.formFocus == 0, m.joinInput.View(), w),
		row(m.passEye(idJoinEye)),
		"",
	}
	switch {
	case m.connecting:
		lines = append(lines, m.spin.View()+infoTextStyle.Render(" Bağlanılıyor..."), "")
	case m.errText != "":
		lines = append(lines, errorStyle.Render("✕ "+m.errText), "")
	}
	lines = append(lines, row(m.btn(idJoinGo, "Bağlan", btnPrimary), m.btn(idJoinCancel, "İptal", btnNormal)))
	body := m.modal("Odaya katıl", lines...)
	return m.frame("", body, m.toastLine(),
		lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, hint("Enter", "bağlan", "Ctrl+E", "göster", "Esc", "iptal")), true)
}

func (m model) viewSettings() string {
	label := func(i int, s string) string {
		if m.settingsCursor == i {
			return keyStyle.Render("▸ ") + lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("%-16s", s))
		}
		return "  " + fmt.Sprintf("%-16s", s)
	}

	nameVal := lipgloss.JoinHorizontal(lipgloss.Center, m.username+"  ", m.btn(idSetName, "Düzenle", btnNormal))
	if m.editingName {
		nameVal = inputBox(true, m.usernameInput.View(), 36)
	}

	swatches := make([]string, 0, len(themes)*2)
	for i, t := range themes {
		id := idSetTheme + strconv.Itoa(i)
		dot := lipgloss.NewStyle().Foreground(t.primary).Render("●")
		txt := " " + dot + " " + t.name + " "
		st := lipgloss.NewStyle().Padding(0, 1)
		switch {
		case t.name == m.settings.Theme:
			st = st.Bold(true).Background(lipgloss.Color("#2b3040")).Foreground(lipgloss.Color("#ffffff"))
			txt = " " + dot + " " + t.name + " ✓"
		case m.hover == id:
			st = st.Background(lipgloss.Color("#46506d")).Foreground(lipgloss.Color("#ffffff"))
		}
		swatches = append(swatches, zone.Mark(id, st.Render(txt)), " ")
	}

	timeLabel := "Kapalı"
	if m.settings.ShowTime {
		timeLabel = "Açık"
	}
	timeKind := btnNormal
	if m.settings.ShowTime {
		timeKind = btnPrimary
	}

	body := m.modal("Ayarlar",
		label(0, "Kullanıcı adı")+nameVal,
		"",
		label(1, "Tema"),
		"  "+lipgloss.JoinHorizontal(lipgloss.Center, swatches...),
		"",
		label(2, "Zaman damgası")+m.btn(idSetTime, timeLabel, timeKind),
		"",
		m.btn(idSetBack, "Geri", btnNormal),
	)
	h := hint("↑↓", "seç", "←→", "tema", "Enter", "değiştir", "Esc", "geri")
	if m.editingName {
		h = hint("Enter", "kaydet", "Esc", "iptal")
	}
	return m.frame("", body, m.toastLine(), lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, h), true)
}

func (m model) viewHelp() string {
	k := func(a, b string) string { return keyStyle.Render(fmt.Sprintf("%-14s", a)) + b }
	body := m.modal("Yardım",
		titleTextStyle.Render("Ana ekran"),
		k("N", "yeni oda kur"),
		k("↑↓ / Enter", "bir odaya katıl"),
		k("S / ?", "ayarlar / yardım"),
		"",
		titleTextStyle.Render("Sohbet"),
		k("Enter", "mesajı gönder"),
		k("Tab / /file", "dosya seç ve gönder"),
		k("Sürükle-bırak", "dosyayı pencereye bırak, yolu otomatik gönderilir"),
		k("/send <yol>", "yoldaki dosyayı gönder"),
		k("/ara <kelime>", "odanın geçmişinde ara"),
		k("/users", "odadaki kişiler"),
		k("F2", "kişi panelini aç/kapat"),
		k("PgUp / PgDn", "geçmişte kaydır (fare tekerleği de)"),
		k("Esc", "odadan ayrıl"),
		k("Y / N", "dosya teklifini kabul / reddet"),
		"",
		infoTextStyle.Render("Dosyalar '"+Cfg.DownloadDir+"' klasörüne kaydedilir; seçici '"+Cfg.UploadDir+"' klasörünü açar."),
		infoTextStyle.Render("Her dosyanın SHA-256 özeti alıcıda doğrulanır."),
		infoTextStyle.Render("Fare tıklaması çalışmazsa Shift tuşuyla seçim yapabilirsiniz."),
		"",
		m.btn(idHelpBack, "Geri", btnPrimary),
	)
	return m.frame("", body, "", lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Center, hint("Esc", "geri")), true)
}

func (m model) viewPicker() string {
	top := m.topBar(brandStyle.Render("DOSYA SEÇ")+" "+infoTextStyle.Render(m.picker.CurrentDirectory), m.btn(idPickBack, "Geri", btnNormal))
	body := panelStyle.Width(m.contentWidth() - 4).Render(m.picker.View())
	return m.frame(top, body, "", hint("↑↓", "gez", "Enter", "seç / aç", "←", "üst klasör", "Esc", "geri"), true)
}

// ---- sohbet ----

func (m model) typingNames() []string {
	var names []string
	for n, at := range m.typing {
		if n != m.username && time.Since(at) < typingTTL {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

// statusLine: aktarım ilerlemesi > bildirim > bağlantı kopması > yazıyor bilgisi.
func (m model) statusLine() string {
	if len(m.transfers) > 0 {
		keys := make([]string, 0, len(m.transfers))
		for k := range m.transfers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		tr := m.transfers[keys[0]]
		s := infoTextStyle.Render(tr.label) + " " + m.progressBar.ViewAs(tr.frac)
		if len(keys) > 1 {
			s += infoTextStyle.Render(fmt.Sprintf("  (+%d aktarım)", len(keys)-1))
		}
		return s
	}
	if t := m.toastLine(); t != "" {
		return t
	}
	if m.lostConn {
		return errorStyle.Render("● Bağlantı koptu")
	}
	if names := m.typingNames(); len(names) > 0 {
		return msgSystemStyle.Render("✎ " + strings.Join(names, ", ") + " yazıyor...")
	}
	return ""
}

func (m model) sidebarView(height int) string {
	users := m.memberList()
	var b strings.Builder
	b.WriteString(keyStyle.Render(fmt.Sprintf("Odada (%d)", len(users))) + "\n\n")
	typing := map[string]bool{}
	for _, n := range m.typingNames() {
		typing[n] = true
	}
	for i, u := range users {
		line := senderStyle(u).Render(u)
		if u == m.username {
			line = msgUserStyle.Render(u) + infoTextStyle.Render(" (sen)")
		}
		if i == 0 {
			line = "★ " + line // host
		} else {
			line = "  " + line
		}
		if typing[u] {
			line += msgSystemStyle.Render(" ✎")
		}
		b.WriteString(line + "\n")
	}
	if m.isHosting && m.hostPort > 0 {
		b.WriteString("\n" + infoTextStyle.Render("Adres") + "\n")
		b.WriteString(fmt.Sprintf("%s:%d\n\n", m.hostIP, m.hostPort))
		b.WriteString(m.btn(idChatCopy, "Kopyala", btnNormal))
	}
	return panelStyle.Width(sidebarWidth - 4).Height(height).Render(b.String())
}

func (m model) viewChat() string {
	role := "katılımcı"
	if m.isHosting {
		role = "host"
	}
	left := brandStyle.Render("VIAN") + " " + lipgloss.NewStyle().Bold(true).Render(m.roomName) + "  " +
		infoTextStyle.Render(fmt.Sprintf("%d kişi · %s · AES-256-GCM", len(m.memberList()), role))
	right := row(
		m.btn(idChatFile, "Dosya", btnNormal),
		m.btn(idChatSearch, "Ara", btnNormal),
		m.btn(idChatUsers, "Kişiler", btnNormal),
		m.btn(idChatLeave, "Ayrıl", btnDanger),
	)
	top := m.topBar(left, right)

	box := panelStyle.Render(m.viewport.View())
	if m.sidebarVisible() {
		box = lipgloss.JoinHorizontal(lipgloss.Top, box, " ", m.sidebarView(m.viewport.Height))
	}

	var composer string
	switch {
	case len(m.offers) > 0:
		o := m.offers[0]
		extra := ""
		if len(m.offers) > 1 {
			extra = fmt.Sprintf("  (+%d bekliyor)", len(m.offers)-1)
		}
		composer = modalStyle.Width(m.contentWidth() - 6).Height(3).Render(
			lipgloss.NewStyle().Bold(true).Render(o.node.Name+" dosya göndermek istiyor") + "\n" +
				fmt.Sprintf("%s (%s)%s", o.offer.FileName, formatSize(o.offer.FileSize), extra) + "\n" +
				row(m.btn(idOfferYes, "Kabul Et (Y)", btnPrimary), m.btn(idOfferNo, "Reddet (N)", btnDanger)))
	case m.confirmLeave:
		composer = modalStyle.Width(m.contentWidth() - 6).Height(3).Render(
			lipgloss.NewStyle().Bold(true).Render("Odadan ayrılmak istiyor musunuz?") + "\n" +
				infoTextStyle.Render("Bağlantı kapanır; odayı sen kurduysan oda da kapanır.") + "\n" +
				row(m.btn(idLeaveYes, "Evet, ayrıl (Y)", btnDanger), m.btn(idLeaveNo, "Vazgeç (N)", btnNormal)))
	default:
		send := lipgloss.NewStyle().Height(5).Width(sendBtnWidth).Align(lipgloss.Center, lipgloss.Center).
			Bold(true).Foreground(lipgloss.Color("#000000")).Background(primaryColor)
		if m.hover == idChatSend {
			send = send.Background(secondaryColor)
		}
		composer = lipgloss.JoinHorizontal(lipgloss.Top,
			panelStyle.Render(m.textarea.View()), " ", zone.Mark(idChatSend, send.Render("Gönder ➤")))
	}

	h := hint("Enter", "gönder", "Tab", "dosya", "sürükle-bırak", "dosya gönder", "F2", "kişiler", "Esc", "ayrıl")
	if strings.HasPrefix(m.textarea.Value(), "/") {
		h = hint("/help", "yardım", "/file", "dosya", "/send <yol>", "gönder", "/ara", "ara", "/users", "kişiler")
	}
	body := lipgloss.JoinVertical(lipgloss.Left, box, composer)
	return m.frame(top, body, m.statusLine(), h, false)
}
