package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"vian/network"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

func press(m tea.Model, k tea.KeyType) tea.Model {
	m, _ = m.Update(tea.KeyMsg{Type: k})
	return m
}

func typeText(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func testModel(t *testing.T) model {
	t.Helper()
	Cfg.DataDir = t.TempDir()
	Cfg.DownloadDir = t.TempDir()
	Cfg.UploadDir = t.TempDir()
	return initialModel()
}

func homeModel(t *testing.T) tea.Model {
	t.Helper()
	var tm tea.Model = testModel(t)
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	tm = typeText(tm, "Memo")
	tm = press(tm, tea.KeyEnter)
	if v := tm.View(); !strings.Contains(v, "Oda Kur") {
		t.Fatalf("ana ekran görünmüyor:\n%s", v)
	}
	return tm
}

func TestScreensRender(t *testing.T) {
	tm := homeModel(t)

	for key, want := range map[string]string{"n": "Yeni oda kur", "s": "Ayarlar", "?": "Yardım"} {
		tm = typeText(tm, key)
		if v := tm.View(); !strings.Contains(v, want) {
			t.Fatalf("%q sonrası %q görünmüyor:\n%s", key, want, v)
		}
		tm = press(tm, tea.KeyEsc)
		if v := tm.View(); !strings.Contains(v, "Ağdaki odalar") {
			t.Fatalf("Esc ana ekrana dönmedi:\n%s", v)
		}
	}
}

// bounds zone kaydı asenkron işlendiği için kısa süre bekler.
func bounds(t *testing.T, id string) (x, y int) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if z := zone.Get(id); z != nil && !z.IsZero() {
			return z.StartX, z.StartY
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s bölgesi bulunamadı", id)
	return 0, 0
}

func TestMouseClickAndHover(t *testing.T) {
	tm := homeModel(t)
	tm.View()
	x, y := bounds(t, idHomeHost)

	// Üzerine gelince vurgulanır
	tm, _ = tm.Update(tea.MouseMsg{X: x + 1, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonNone})
	if tm.(model).hover != idHomeHost {
		t.Fatalf("hover = %q", tm.(model).hover)
	}

	// Tıklayınca oda kurma formu açılır
	tm, _ = tm.Update(tea.MouseMsg{X: x + 1, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	if tm.(model).state != stateHostForm {
		t.Fatalf("tıklama formu açmadı, state = %v", tm.(model).state)
	}
	if v := tm.View(); !strings.Contains(v, "Rastgele parola") {
		t.Fatalf("form görünmüyor:\n%s", v)
	}
}

func TestHostFormValidationAndRandomPassword(t *testing.T) {
	tm := homeModel(t)
	tm = typeText(tm, "n")
	tm = press(tm, tea.KeyEnter) // oda adı alanından parola alanına geçer
	tm = press(tm, tea.KeyTab)   // Oda Kur butonu
	tm = press(tm, tea.KeyEnter) // ad boş -> hata
	if v := tm.View(); !strings.Contains(v, "Oda adı boş olamaz") {
		t.Fatalf("doğrulama hatası görünmüyor:\n%s", v)
	}

	mm := tm.(model)
	mm.action(idHostRand)
	if p := mm.hostInput.Value(); len(p) != 11 || p[5] != '-' {
		t.Fatalf("rastgele parola biçimi: %q", p)
	}
}

func TestSettingsPersist(t *testing.T) {
	tm := homeModel(t)
	mm := tm.(model)
	mm.action(idSetTheme + "1")
	mm.action(idSetTime)

	s := loadSettings()
	if s.Theme != themes[1].name || s.Username != "Memo" || s.ShowTime {
		t.Fatalf("ayarlar kaydedilmedi: %+v", s)
	}
	applyTheme(0)
}

func TestPeersLiveListAndPrune(t *testing.T) {
	tm := homeModel(t)
	mm := tm.(model)
	mm.joinStop = make(chan struct{}) // tarama açık sayılsın
	mm.upsertPeer(peerFoundMsg{ip: "10.0.0.5", msg: network.DiscoveryMessage{Port: 5000, HostName: "Karargah", Owner: "Ali"}})
	mm.upsertPeer(peerFoundMsg{ip: "10.0.0.5", msg: network.DiscoveryMessage{Port: 5000, HostName: "Karargah", Owner: "Ali"}}) // aynı oda tekrar
	if len(mm.foundPeers) != 1 {
		t.Fatalf("aynı oda tekrarlandı: %d", len(mm.foundPeers))
	}
	if v := mm.View(); !strings.Contains(v, "Karargah") {
		t.Fatalf("oda listede yok:\n%s", v)
	}

	mm.pruneNow(time.Now().Add(peerTTL + time.Second))
	if len(mm.foundPeers) != 0 {
		t.Fatal("süresi dolan oda silinmedi")
	}
}

func TestChatFlow(t *testing.T) {
	m := testModel(t)
	m.username = "Memo"
	m.roomName = "Oda"
	m.isHosting = true
	m.width, m.height = 120, 40
	m.enterChat()

	m.addChat("Memo", "merhaba dünya")
	m.addSystem("deneme")
	v := m.View()
	if !strings.Contains(v, "merhaba") || !strings.Contains(v, "Oda") || !strings.Contains(v, "Gönder") {
		t.Fatalf("sohbet görünmüyor:\n%s", v)
	}

	// Dar ekranda kişi paneli gizlenir, yine de çizilir.
	m.width, m.height = 60, 20
	m.resize()
	_ = m.View()

	m.runInput("/foo")
	m.runInput("/users")
	m.runInput("/ara")
	if len(m.lines) < 4 {
		t.Fatalf("komut çıktıları eklenmedi: %d", len(m.lines))
	}

	// Ayrılma onayı penceresi
	m.action(idChatLeave)
	if v := m.View(); !strings.Contains(v, "Odadan ayrılmak istiyor musunuz") {
		t.Fatalf("onay penceresi yok:\n%s", v)
	}
	m.action(idLeaveNo)

	// Gelen dosya teklifi penceresi
	m.offers = []incomingOffer{{offer: network.FileOffer{FileID: "x", FileName: "a.txt", FileSize: 2048}, node: &network.Node{Name: "Ali"}}}
	if v := m.View(); !strings.Contains(v, "a.txt") || !strings.Contains(v, "Kabul Et") {
		t.Fatalf("teklif penceresi yok:\n%s", v)
	}
}

// connectedNodes gerçek bir oda kurup bağlanır; host tarafındaki düğümü döndürür.
func connectedNode(t *testing.T) *network.Node {
	t.Helper()
	room, err := network.NewRoom("oda", "host", "gizli")
	if err != nil {
		t.Fatal(err)
	}
	joined := make(chan *network.Node, 1)
	l, port, err := room.Listen(0, func(n *network.Node) { joined <- n })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	client, _, err := network.Connect(fmt.Sprintf("127.0.0.1:%d", port), "gizli", room.Salt, "Ali")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	select {
	case n := <-joined:
		t.Cleanup(func() { n.Close() })
		return client // istemci düğümü: ona yazınca host alır
	case <-time.After(3 * time.Second):
		t.Fatal("bağlantı kurulamadı")
		return nil
	}
}

func TestDragDropSendsFile(t *testing.T) {
	node := connectedNode(t)

	file := filepath.Join(t.TempDir(), "rapor dosyası.txt")
	os.WriteFile(file, []byte("içerik"), 0644)

	m := testModel(t)
	m.username = "Memo"
	m.roomName = "Oda"
	m.width, m.height = 110, 30
	m.nodes = []*network.Node{node}
	m.enterChat()

	// 1) Bracketed paste olarak gelen bırakma
	var tm tea.Model = m
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(`"` + file + `"`), Paste: true})
	got := tm.(model)
	if len(got.outgoing) != 1 {
		t.Fatalf("dosya teklifi oluşmadı: %v", got.outgoing)
	}
	if v := got.textarea.Value(); v != "" {
		t.Fatalf("yol mesaj kutusuna yazılmamalı: %q", v)
	}

	// 2) Karakter karakter gelip Enter ile onaylanan bırakma
	tm = typeText(tm, file)
	tm = press(tm, tea.KeyEnter)
	if n := len(tm.(model).outgoing); n != 2 {
		t.Fatalf("Enter ile gönderilmedi, teklif sayısı: %d", n)
	}

	// 3) Sıradan yapıştırma mesaj kutusuna normal gider
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("sadece metin"), Paste: true})
	if v := tm.(model).textarea.Value(); !strings.Contains(v, "sadece metin") {
		t.Fatalf("normal yapıştırma kayboldu: %q", v)
	}
}
