package ui

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"vian/database"
	"vian/network"
	"vian/transfer"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type uiState int

const (
	stateUserSetup uiState = iota
	stateHome
	stateHostForm
	stateJoinForm
	stateChat
	stateFilePicker
	stateHelp
	stateSettings
)

type lineKind int

const (
	lineChat lineKind = iota
	lineSystem
	lineDivider
)

type chatLine struct {
	at     string
	sender string
	text   string
	kind   lineKind
}

type incomingPacketMsg struct {
	packet *network.Packet
	node   *network.Node
}

type peerLeftMsg struct{ node *network.Node }

type peerFoundMsg struct {
	ip  string
	msg network.DiscoveryMessage
}

type foundPeer struct {
	peerFoundMsg
	seen time.Time
}

type connectionEstablishedMsg struct {
	node *network.Node
	ack  *network.HandshakeAck // yalnızca istemci tarafında dolu
}

type hostStartedMsg struct {
	listener net.Listener
	port     int
}

type fileProgressMsg struct {
	key     string
	percent float64
}

type fileFinishedMsg struct {
	key   string
	label string
	err   error
}

type connectionErrorMsg struct{ err error }
type scanErrorMsg struct{ err error }
type tickMsg time.Time

const (
	chatReserved = 11 // üst çubuk, kenarlıklar, giriş alanı, durum ve ipucu satırları
	sidebarWidth = 26
	typingTTL    = 4 * time.Second
	peerTTL      = 8 * time.Second
	sendBtnWidth = 12
)

type activeTransfer struct {
	label string
	frac  float64
	node  *network.Node
	name  string
	total int64
	got   int64
}

type incomingOffer struct {
	offer network.FileOffer
	node  *network.Node
}

type model struct {
	state uiState

	settings       Settings
	settingsCursor int
	editingName    bool

	// Kullanıcı
	usernameInput textinput.Model
	username      string
	localAddr     string

	// Genel etkileşim
	hover      string // fare üzerindeki buton
	focus      int    // ana ekranda odaklı öğe
	formFocus  int    // formlarda odaklı alan/buton
	showPass   bool
	toast      string
	toastErr   bool
	toastUntil time.Time
	now        time.Time
	spin       spinner.Model

	// Oda kurma
	hostNameInput textinput.Model
	roomName      string
	hostInput     textinput.Model
	isHosting     bool
	hostPort      int
	hostIP        string
	hostStop      chan struct{}
	listener      net.Listener

	// Odaya katılma
	foundPeers  []foundPeer
	joinInput   textinput.Model
	joiningPeer *peerFoundMsg
	connecting  bool
	errText     string
	joinStop    chan struct{}
	scanErr     string

	// Sohbet
	viewport     viewport.Model
	textarea     textarea.Model
	lines        []chatLine
	nodes        []*network.Node // Çoklu bağlantı için (Grup Sohbeti)
	members      []string        // istemci tarafında host'tan gelen kişi listesi
	typing       map[string]time.Time
	lastTyping   time.Time
	showSidebar  bool
	confirmLeave bool
	lostConn     bool
	picker       filepicker.Model

	// Dosya aktarımı
	offers      []incomingOffer
	outgoing    map[string]string // teklif kimliği -> yerel dosya yolu
	transfers   map[string]*activeTransfer
	progressBar progress.Model

	discoveryChan chan peerFoundMsg

	width  int
	height int
}

func newInput(placeholder string, limit int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = limit
	ti.Width = 32
	return ti
}

func initialModel() model {
	zone.NewGlobal()
	settings := loadSettings()
	applyTheme(themeIndex(settings.Theme))

	uInput := newInput("Kullanıcı adınız (örn: Memo)", 20)
	uInput.Focus()

	hNameInput := newInput("Oda adı (örn: Gizli Karargah)", 30)
	hostPass := newInput("Oda parolası", 32)
	joinPass := newInput("Oda parolası", 32)
	for _, in := range []*textinput.Model{&hostPass, &joinPass} {
		in.EchoMode = textinput.EchoPassword
		in.EchoCharacter = '•'
	}

	ta := textarea.New()
	ta.Placeholder = "Mesaj yazın veya bir dosyayı buraya sürükleyip bırakın..."
	ta.Focus()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 280
	ta.SetWidth(60)
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.KeyMap.InsertNewline.SetEnabled(false) // Enter mesaj gönderir

	vp := viewport.New(60, 15)
	// Harfler (j/k/d/u...) yazarken kaydırmasın: yalnızca PgUp/PgDn ve fare tekerleği.
	vp.KeyMap = viewport.KeyMap{
		PageDown: key.NewBinding(key.WithKeys("pgdown")),
		PageUp:   key.NewBinding(key.WithKeys("pgup")),
	}

	pb := progress.New(progress.WithDefaultGradient())
	pb.Width = 20

	os.MkdirAll(Cfg.UploadDir, os.ModePerm)
	os.MkdirAll(Cfg.DownloadDir, os.ModePerm)
	absSendDir, _ := filepath.Abs(Cfg.UploadDir)

	fp := filepicker.New()
	fp.CurrentDirectory = absSendDir
	fp.Height = 12

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot

	m := model{
		state:         stateUserSetup,
		settings:      settings,
		usernameInput: uInput,
		hostNameInput: hNameInput,
		hostInput:     hostPass,
		joinInput:     joinPass,
		textarea:      ta,
		viewport:      vp,
		progressBar:   pb,
		picker:        fp,
		spin:          sp,
		showSidebar:   true,
		typing:        map[string]time.Time{},
		outgoing:      map[string]string{},
		transfers:     map[string]*activeTransfer{},
		now:           time.Now(),
		localAddr:     localIP(),
	}
	if settings.Username != "" {
		m.username = settings.Username
		m.state = stateHome // tarama ilk mesajda (keyHome) başlar
	}
	return m
}

func listenForPackets(node *network.Node) tea.Cmd {
	return func() tea.Msg {
		if node == nil {
			return nil
		}
		packet, ok := <-node.Incoming
		if !ok {
			return peerLeftMsg{node: node}
		}
		return incomingPacketMsg{packet: packet, node: node}
	}
}

func waitForDiscovery(ch chan peerFoundMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd(), m.spin.Tick)
}

// ---- tarama ----

// startScan ağdaki odaları dinlemeye başlar (zaten çalışıyorsa bir şey yapmaz).
func (m *model) startScan() []tea.Cmd {
	if m.joinStop != nil {
		return nil
	}
	m.scanErr = ""
	m.joinStop = make(chan struct{})
	ch := make(chan peerFoundMsg, 100)
	m.discoveryChan = ch
	stop := m.joinStop
	go func() {
		err := network.ListenForPeers(func(ip string, pMsg network.DiscoveryMessage) {
			select {
			case ch <- peerFoundMsg{ip: ip, msg: pMsg}:
			default:
			}
		}, stop)
		close(ch) // dinleme bitti, bekleyen okuyucular sonlansın
		if err != nil && GlobalProgram != nil {
			GlobalProgram.Send(scanErrorMsg{err: err})
		}
	}()
	return []tea.Cmd{waitForDiscovery(ch)}
}

func (m *model) stopScan() {
	if m.joinStop != nil {
		close(m.joinStop)
		m.joinStop = nil
	}
	m.foundPeers = nil
}

func (m *model) pruneNow(now time.Time) {
	kept := m.foundPeers[:0]
	for _, p := range m.foundPeers {
		if now.Sub(p.seen) < peerTTL {
			kept = append(kept, p)
		}
	}
	m.foundPeers = kept
	if last := len(homeIDs(m)) - 1; m.focus > last {
		m.focus = last
	}
}

// ---- yaşam döngüsü yardımcıları ----

// closeHosting dinleyiciyi ve yayını kapatır.
func (m *model) closeHosting() {
	if m.hostStop != nil {
		close(m.hostStop)
		m.hostStop = nil
	}
	if m.listener != nil {
		m.listener.Close()
		m.listener = nil
	}
}

func (m *model) shutdown() {
	m.stopScan()
	m.closeHosting()
	for _, n := range m.nodes {
		n.Close()
	}
}

// leaveRoom sohbetten çıkıp ana ekrana döner.
func (m *model) leaveRoom() []tea.Cmd {
	m.shutdown()
	m.nodes = nil
	m.members = nil
	m.lines = nil
	m.offers = nil
	m.outgoing = map[string]string{}
	m.transfers = map[string]*activeTransfer{}
	m.typing = map[string]time.Time{}
	m.isHosting = false
	m.hostPort = 0
	m.joiningPeer = nil
	m.connecting = false
	m.confirmLeave = false
	m.lostConn = false
	m.errText = ""
	m.hostInput.SetValue("")
	m.hostNameInput.SetValue("")
	m.joinInput.SetValue("")
	m.textarea.Reset()
	m.focus = 0
	m.state = stateHome
	return m.startScan()
}

func (m *model) memberList() []string {
	if m.isHosting {
		l := []string{m.username}
		for _, n := range m.nodes {
			l = append(l, n.Name)
		}
		return l
	}
	return m.members
}

func (m *model) broadcastUsers() {
	list := network.UserList{Users: m.memberList()}
	for _, n := range m.nodes {
		n.Send(network.MsgTypeUsers, list)
	}
}

func (m *model) removeNode(n *network.Node) bool {
	for i, x := range m.nodes {
		if x == n {
			m.nodes = append(m.nodes[:i], m.nodes[i+1:]...)
			n.Close()
			for k, tr := range m.transfers {
				if tr.node == n {
					delete(m.transfers, k)
				}
			}
			return true
		}
	}
	return false
}

func (m *model) hasNode(n *network.Node) bool {
	for _, x := range m.nodes {
		if x == n {
			return true
		}
	}
	return false
}

func (m *model) setToast(text string, isErr bool) {
	m.toast, m.toastErr = text, isErr
	m.toastUntil = time.Now().Add(4 * time.Second)
}

// ---- yerleşim ----

func (m *model) contentWidth() int {
	w := m.width - 4
	if m.width == 0 || w > 110 {
		w = 110
	}
	if w < 40 {
		w = 40
	}
	return w
}

func (m *model) sidebarVisible() bool {
	return m.showSidebar && m.width >= 90
}

func (m *model) resize() {
	cw := m.contentWidth()
	chatW := cw
	if m.sidebarVisible() {
		chatW = cw - sidebarWidth - 1
	}
	m.viewport.Width = chatW - 4
	m.textarea.SetWidth(cw - sendBtnWidth - 1 - 6)
	h := m.height - chatReserved
	if m.height == 0 || h < 5 {
		h = 12
	}
	m.viewport.Height = h
	m.refreshViewport()
}

func (m *model) enterChat() {
	m.stopScan()
	m.state = stateChat
	m.textarea.Reset()
	m.textarea.Focus()
	m.lines = nil
	m.loadHistory()
	m.resize()
}

// ---- Update ----

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Sürükle-bırak: terminal bırakılan dosyanın yolunu yapıştırılmış metin olarak iletir.
	if k, ok := msg.(tea.KeyMsg); ok && m.state == stateChat && len(m.offers) == 0 && !m.confirmLeave &&
		(k.Paste || (k.Type == tea.KeyRunes && len(k.Runes) > 1)) {
		if paths := parsePaths(string(k.Runes)); paths != nil {
			m.sendFiles(paths)
			return m, nil
		}
	}

	// Giriş alanı ve kaydırma: yalnızca sohbetteyken ve bir pencere açık değilken.
	if m.state == stateChat {
		_, isKey := msg.(tea.KeyMsg)
		if !(isKey && (len(m.offers) > 0 || m.confirmLeave)) {
			var taCmd, vpCmd tea.Cmd
			m.textarea, taCmd = m.textarea.Update(msg)
			m.viewport, vpCmd = m.viewport.Update(msg)
			cmds = append(cmds, taCmd, vpCmd)
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.picker.Height = max(5, msg.Height-12)

	case spinner.TickMsg:
		var c tea.Cmd
		m.spin, c = m.spin.Update(msg)
		cmds = append(cmds, c)

	case tea.MouseMsg:
		m.hover = ""
		for _, id := range visibleIDs(&m) {
			if z := zone.Get(id); z != nil && !z.IsZero() && z.InBounds(msg) {
				m.hover = id
				break
			}
		}
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft && m.hover != "" {
			cmds = append(cmds, m.action(m.hover)...)
		}

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.shutdown()
			return m, tea.Quit
		}

	case hostStartedMsg:
		if !m.connecting { // vazgeçildi
			msg.listener.Close()
			break
		}
		m.connecting = false
		m.isHosting = true
		m.listener = msg.listener
		m.hostPort = msg.port
		m.hostIP = m.localAddr
		m.enterChat()
		m.addSystem(fmt.Sprintf("'%s' odası hazır. Adres: %s:%d", m.roomName, m.hostIP, m.hostPort))
		m.addSystem("Katılımcılar odanı \"Ağdaki odalar\" listesinde görecek; parolayı onlara sen ilet.")

	case scanErrorMsg:
		m.scanErr = "Ağ taraması başlatılamadı: " + msg.err.Error()

	case connectionErrorMsg:
		m.connecting = false
		m.errText = msg.err.Error()
		if m.state == stateHostForm {
			m.closeHosting()
		} else {
			m.joinInput.SetValue("")
		}

	case connectionEstablishedMsg:
		if !m.isHosting && msg.ack == nil {
			msg.node.Close() // oda kapatıldıktan sonra gelen bağlantı
			break
		}
		m.nodes = append(m.nodes, msg.node)
		if !m.isHosting {
			m.connecting = false
		}
		if m.state != stateChat && m.state != stateFilePicker {
			m.enterChat()
		}
		if m.isHosting {
			m.addSystem(fmt.Sprintf("%s odaya katıldı. (Odada %d kişi)", msg.node.Name, len(m.nodes)+1))
			m.broadcastUsers()
		} else {
			m.roomName = msg.ack.Room
			m.members = []string{msg.ack.Owner, m.username}
			m.addSystem(fmt.Sprintf("'%s' odasına bağlanıldı.", msg.ack.Room))
		}
		cmds = append(cmds, listenForPackets(msg.node))

	case peerLeftMsg:
		if !m.removeNode(msg.node) {
			break
		}
		if m.isHosting {
			m.addSystem(fmt.Sprintf("%s odadan ayrıldı. (Odada %d kişi)", msg.node.Name, len(m.nodes)+1))
			m.broadcastUsers()
		} else if len(m.nodes) == 0 {
			m.members = nil
			m.lostConn = true
			m.addSystem("Bağlantı koptu. Odadan ayrılıp tekrar katılabilirsiniz.")
		}

	case incomingPacketMsg:
		if m.hasNode(msg.node) {
			m.handlePacket(msg.packet, msg.node)
			cmds = append(cmds, listenForPackets(msg.node))
		}

	case fileProgressMsg:
		if tr, ok := m.transfers[msg.key]; ok {
			tr.frac = msg.percent
		}

	case fileFinishedMsg:
		delete(m.transfers, msg.key)
		if msg.err != nil {
			m.addSystem(fmt.Sprintf("Gönderim başarısız (%s): %v", msg.label, msg.err))
		} else {
			m.addSystem("Gönderildi: " + msg.label)
		}

	case peerFoundMsg:
		if m.joinStop != nil {
			m.upsertPeer(msg)
			cmds = append(cmds, waitForDiscovery(m.discoveryChan))
		}

	case tickMsg:
		m.now = time.Time(msg)
		m.pruneNow(m.now)
		if m.toast != "" && m.now.After(m.toastUntil) {
			m.toast = ""
		}
		cmds = append(cmds, tickCmd())
	}

	switch m.state {
	case stateUserSetup:
		cmds = append(cmds, m.keyUserSetup(msg)...)
	case stateHome:
		cmds = append(cmds, m.keyHome(msg)...)
	case stateHelp:
		if k, ok := msg.(tea.KeyMsg); ok && (k.Type == tea.KeyEsc || k.Type == tea.KeyEnter) {
			m.state = stateHome
		}
	case stateSettings:
		cmds = append(cmds, m.updateSettings(msg)...)
	case stateHostForm:
		cmds = append(cmds, m.keyHostForm(msg)...)
	case stateJoinForm:
		cmds = append(cmds, m.keyJoinForm(msg)...)
	case stateChat:
		if k, ok := msg.(tea.KeyMsg); ok {
			cmds = append(cmds, m.updateChatKey(k)...)
		}
	case stateFilePicker:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		cmds = append(cmds, cmd)

		if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
			m.sendFile(path)
			m.state = stateChat
		}
		if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyEsc {
			m.state = stateChat
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) upsertPeer(p peerFoundMsg) {
	now := time.Now()
	for i := range m.foundPeers {
		if m.foundPeers[i].ip == p.ip && m.foundPeers[i].msg.Port == p.msg.Port {
			m.foundPeers[i].seen = now
			m.foundPeers[i].msg = p.msg
			return
		}
	}
	m.foundPeers = append(m.foundPeers, foundPeer{peerFoundMsg: p, seen: now})
	sort.SliceStable(m.foundPeers, func(i, j int) bool {
		return m.foundPeers[i].msg.HostName < m.foundPeers[j].msg.HostName
	})
}

// ---- tıklanabilir öğeler ----

const (
	idSetupGo    = "setup:go"
	idHomeHost   = "home:host"
	idHomeSet    = "home:settings"
	idHomeHelp   = "home:help"
	idHomeQuit   = "home:quit"
	idHomeJoin   = "home:join:"
	idHostCreate = "host:create"
	idHostCancel = "host:cancel"
	idHostRand   = "host:rand"
	idHostEye    = "host:eye"
	idJoinGo     = "join:go"
	idJoinCancel = "join:cancel"
	idJoinEye    = "join:eye"
	idChatSend   = "chat:send"
	idChatFile   = "chat:file"
	idChatSearch = "chat:search"
	idChatUsers  = "chat:users"
	idChatLeave  = "chat:leave"
	idChatCopy   = "chat:copy"
	idLeaveYes   = "leave:yes"
	idLeaveNo    = "leave:no"
	idOfferYes   = "offer:yes"
	idOfferNo    = "offer:no"
	idSetName    = "set:name"
	idSetTheme   = "set:theme:"
	idSetTime    = "set:time"
	idSetBack    = "set:back"
	idHelpBack   = "help:back"
	idPickBack   = "picker:back"
)

// homeIDs ana ekrandaki odaklanabilir öğeleri (araç çubuğu + odalar) sırayla döndürür.
func homeIDs(m *model) []string {
	ids := []string{idHomeHost, idHomeSet, idHomeHelp, idHomeQuit}
	for i := range m.foundPeers {
		ids = append(ids, idHomeJoin+strconv.Itoa(i))
	}
	return ids
}

// visibleIDs o an ekranda tıklanabilir olan öğeleri döndürür.
func visibleIDs(m *model) []string {
	switch m.state {
	case stateUserSetup:
		return []string{idSetupGo}
	case stateHome:
		return homeIDs(m)
	case stateHostForm:
		return []string{idHostCreate, idHostCancel, idHostRand, idHostEye}
	case stateJoinForm:
		return []string{idJoinGo, idJoinCancel, idJoinEye}
	case stateSettings:
		ids := []string{idSetName, idSetTime, idSetBack}
		for i := range themes {
			ids = append(ids, idSetTheme+strconv.Itoa(i))
		}
		return ids
	case stateHelp:
		return []string{idHelpBack}
	case stateFilePicker:
		return []string{idPickBack}
	case stateChat:
		switch {
		case len(m.offers) > 0:
			return []string{idOfferYes, idOfferNo}
		case m.confirmLeave:
			return []string{idLeaveYes, idLeaveNo}
		}
		return []string{idChatSend, idChatFile, idChatSearch, idChatUsers, idChatLeave, idChatCopy}
	}
	return nil
}

// action bir butona basılınca (tıklama veya Enter) çalışır.
func (m *model) action(id string) []tea.Cmd {
	var cmds []tea.Cmd
	switch {
	case id == idSetupGo:
		m.submitUsername()
	case id == idHomeHost:
		m.state = stateHostForm
		m.formFocus = 0
		m.errText = ""
		m.hostNameInput.Focus()
		m.hostInput.Blur()
		cmds = append(cmds, textinput.Blink)
	case id == idHomeSet:
		m.state = stateSettings
		m.settingsCursor = 0
	case id == idHomeHelp:
		m.state = stateHelp
	case id == idHomeQuit:
		m.shutdown()
		cmds = append(cmds, tea.Quit)
	case strings.HasPrefix(id, idHomeJoin):
		i, _ := strconv.Atoi(strings.TrimPrefix(id, idHomeJoin))
		if i >= 0 && i < len(m.foundPeers) {
			p := m.foundPeers[i].peerFoundMsg
			m.joiningPeer = &p
			m.state = stateJoinForm
			m.formFocus = 0
			m.errText = ""
			m.joinInput.SetValue("")
			m.joinInput.Focus()
			cmds = append(cmds, textinput.Blink)
		}
	case id == idHostCancel:
		m.cancelHostForm()
	case id == idHostEye || id == idJoinEye:
		m.togglePass()
	case id == idHostRand:
		m.hostInput.SetValue(randomPassword())
		m.showPass = true
		m.applyEcho()
		m.formFocus = 1
		m.focusHostField()
	case id == idHostCreate:
		cmds = append(cmds, m.submitHost()...)
	case id == idJoinCancel:
		m.cancelJoinForm()
	case id == idJoinGo:
		cmds = append(cmds, m.submitJoin()...)
	case id == idSetName, id == idSetTime, strings.HasPrefix(id, idSetTheme), id == idSetBack:
		cmds = append(cmds, m.settingsAction(id)...)
	case id == idHelpBack:
		m.state = stateHome
	case id == idPickBack:
		m.state = stateChat
	case id == idChatSend:
		if v := strings.TrimSpace(m.textarea.Value()); v != "" {
			cmds = append(cmds, m.runInput(v)...)
			m.textarea.Reset()
		}
	case id == idChatFile:
		m.openPicker(&cmds)
	case id == idChatSearch:
		m.textarea.SetValue("/ara ")
		m.textarea.CursorEnd()
		m.textarea.Focus()
	case id == idChatUsers:
		m.showSidebar = !m.showSidebar
		m.resize()
	case id == idChatCopy:
		if m.isHosting && m.hostPort > 0 {
			addr := fmt.Sprintf("%s:%d", m.hostIP, m.hostPort)
			if err := clipboard.WriteAll(addr); err != nil {
				m.setToast("Panoya kopyalanamadı", true)
			} else {
				m.setToast("Adres panoya kopyalandı: "+addr, false)
			}
		}
	case id == idChatLeave:
		m.confirmLeave = true
	case id == idLeaveNo:
		m.confirmLeave = false
	case id == idLeaveYes:
		cmds = append(cmds, m.leaveRoom()...)
	case id == idOfferYes:
		m.answerOffer(true)
	case id == idOfferNo:
		m.answerOffer(false)
	}
	return cmds
}

func (m *model) answerOffer(accept bool) {
	if len(m.offers) == 0 {
		return
	}
	cur := m.offers[0]
	m.offers = m.offers[1:]
	if accept {
		cur.node.Send(network.MsgTypeFileAccept, network.FileAccept{FileID: cur.offer.FileID})
		m.transfers["down:"+cur.offer.FileID] = &activeTransfer{
			label: "↓ " + cur.offer.FileName, node: cur.node, name: cur.offer.FileName, total: cur.offer.FileSize,
		}
		m.addSystem(fmt.Sprintf("Dosya kabul edildi: %s, indiriliyor...", cur.offer.FileName))
		return
	}
	cur.node.Send(network.MsgTypeFileReject, network.FileReject{FileID: cur.offer.FileID})
	m.addSystem("Dosya reddedildi.")
}

// ---- ekran bazlı klavye işleyicileri ----

func (m *model) submitUsername() {
	if v := strings.TrimSpace(m.usernameInput.Value()); v != "" {
		m.username = network.SanitizeName(v)
		m.settings.Username = m.username
		m.settings.save()
		m.state = stateHome
		m.focus = 0
	}
}

func (m *model) keyUserSetup(msg tea.Msg) []tea.Cmd {
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyEnter {
		m.submitUsername()
		return nil
	}
	var c tea.Cmd
	m.usernameInput, c = m.usernameInput.Update(msg)
	return []tea.Cmd{c}
}

func (m *model) keyHome(msg tea.Msg) []tea.Cmd {
	// Ana ekrana ilk girişte (veya odadan dönünce) taramayı başlatır; çalışıyorsa bir şey yapmaz.
	cmds := m.startScan()

	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return cmds
	}
	ids := homeIDs(m)
	switch k.String() {
	case "q":
		m.shutdown()
		return append(cmds, tea.Quit)
	case "n":
		return append(cmds, m.action(idHomeHost)...)
	case "s":
		return append(cmds, m.action(idHomeSet)...)
	case "?", "h":
		return append(cmds, m.action(idHomeHelp)...)
	case "up", "left", "shift+tab":
		if m.focus > 0 {
			m.focus--
		}
	case "down", "right", "tab", "j":
		if m.focus < len(ids)-1 {
			m.focus++
		}
	case "enter", " ":
		if m.focus < len(ids) {
			cmds = append(cmds, m.action(ids[m.focus])...)
		}
	}
	return cmds
}

func (m *model) togglePass() {
	m.showPass = !m.showPass
	m.applyEcho()
}

func (m *model) applyEcho() {
	mode := textinput.EchoPassword
	if m.showPass {
		mode = textinput.EchoNormal
	}
	m.hostInput.EchoMode = mode
	m.joinInput.EchoMode = mode
}

func (m *model) cancelHostForm() {
	if m.connecting {
		m.connecting = false
		m.closeHosting()
	}
	m.state = stateHome
	m.errText = ""
	m.hostInput.SetValue("")
	m.hostNameInput.SetValue("")
}

func (m *model) cancelJoinForm() {
	m.joiningPeer = nil
	m.connecting = false
	m.state = stateHome
	m.errText = ""
	m.joinInput.SetValue("")
}

func (m *model) focusHostField() {
	m.hostNameInput.Blur()
	m.hostInput.Blur()
	switch m.formFocus {
	case 0:
		m.hostNameInput.Focus()
	case 1:
		m.hostInput.Focus()
	}
}

func (m *model) submitHost() []tea.Cmd {
	if m.connecting {
		return nil
	}
	name := strings.TrimSpace(m.hostNameInput.Value())
	password := strings.TrimSpace(m.hostInput.Value())
	switch {
	case name == "":
		m.errText = "Oda adı boş olamaz."
		m.formFocus = 0
		m.focusHostField()
		return nil
	case password == "":
		m.errText = "Parola boş olamaz."
		m.formFocus = 1
		m.focusHostField()
		return nil
	}
	m.errText = ""
	m.roomName = name
	m.connecting = true
	m.hostStop = make(chan struct{})
	return []tea.Cmd{startHostCmd(password, name, m.username, Cfg.Port, m.hostStop)}
}

func (m *model) submitJoin() []tea.Cmd {
	if m.connecting || m.joiningPeer == nil {
		return nil
	}
	password := strings.TrimSpace(m.joinInput.Value())
	if password == "" {
		m.errText = "Parola boş olamaz."
		return nil
	}
	address := fmt.Sprintf("%s:%d", m.joiningPeer.ip, m.joiningPeer.msg.Port)
	m.connecting = true
	m.errText = ""
	return []tea.Cmd{connectCmd(address, password, m.joiningPeer.msg.Salt, m.username)}
}

func (m *model) keyHostForm(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m.updateHostInputs(msg)
	}
	switch k.String() {
	case "esc":
		m.cancelHostForm()
	case "tab", "down":
		m.formFocus = (m.formFocus + 1) % 4
		m.focusHostField()
	case "shift+tab", "up":
		m.formFocus = (m.formFocus + 3) % 4
		m.focusHostField()
	case "ctrl+r":
		cmds = append(cmds, m.action(idHostRand)...)
	case "ctrl+e":
		m.togglePass()
	case "enter":
		switch m.formFocus {
		case 0:
			m.formFocus = 1
			m.focusHostField()
		case 3:
			m.cancelHostForm()
		default:
			cmds = append(cmds, m.submitHost()...)
		}
	default:
		cmds = append(cmds, m.updateHostInputs(msg)...)
	}
	return cmds
}

func (m *model) updateHostInputs(msg tea.Msg) []tea.Cmd {
	var c1, c2 tea.Cmd
	m.hostNameInput, c1 = m.hostNameInput.Update(msg)
	m.hostInput, c2 = m.hostInput.Update(msg)
	return []tea.Cmd{c1, c2}
}

func (m *model) keyJoinForm(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		var c tea.Cmd
		m.joinInput, c = m.joinInput.Update(msg)
		return []tea.Cmd{c}
	}
	switch k.String() {
	case "esc":
		m.cancelJoinForm()
	case "tab", "down":
		m.formFocus = (m.formFocus + 1) % 3
		m.syncJoinFocus()
	case "shift+tab", "up":
		m.formFocus = (m.formFocus + 2) % 3
		m.syncJoinFocus()
	case "ctrl+e":
		m.togglePass()
	case "enter":
		if m.formFocus == 2 {
			m.cancelJoinForm()
		} else {
			cmds = append(cmds, m.submitJoin()...)
		}
	default:
		var c tea.Cmd
		m.joinInput, c = m.joinInput.Update(msg)
		cmds = append(cmds, c)
	}
	return cmds
}

func (m *model) syncJoinFocus() {
	if m.formFocus == 0 {
		m.joinInput.Focus()
	} else {
		m.joinInput.Blur()
	}
}

// ---- ayarlar ----

func (m *model) updateSettings(msg tea.Msg) []tea.Cmd {
	var cmds []tea.Cmd
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		if m.editingName {
			var c tea.Cmd
			m.usernameInput, c = m.usernameInput.Update(msg)
			cmds = append(cmds, c)
		}
		return cmds
	}

	if m.editingName {
		switch k.Type {
		case tea.KeyEnter:
			if v := strings.TrimSpace(m.usernameInput.Value()); v != "" {
				m.username = network.SanitizeName(v)
				m.settings.Username = m.username
				m.settings.save()
				m.setToast("Kullanıcı adı güncellendi", false)
			}
			m.editingName = false
			m.usernameInput.Blur()
		case tea.KeyEsc:
			m.editingName = false
			m.usernameInput.Blur()
		default:
			var c tea.Cmd
			m.usernameInput, c = m.usernameInput.Update(msg)
			cmds = append(cmds, c)
		}
		return cmds
	}

	const rows = 4 // kullanıcı adı, tema, zaman damgası, geri
	switch k.String() {
	case "esc":
		m.state = stateHome
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j":
		if m.settingsCursor < rows-1 {
			m.settingsCursor++
		}
	case "left", "h":
		if m.settingsCursor == 1 {
			m.cycleTheme(-1)
		}
	case "right", "l":
		if m.settingsCursor == 1 {
			m.cycleTheme(1)
		}
	case "enter", " ":
		switch m.settingsCursor {
		case 0:
			cmds = append(cmds, m.settingsAction(idSetName)...)
		case 1:
			m.cycleTheme(1)
		case 2:
			cmds = append(cmds, m.settingsAction(idSetTime)...)
		case 3:
			m.state = stateHome
		}
	}
	return cmds
}

func (m *model) settingsAction(id string) []tea.Cmd {
	var cmds []tea.Cmd
	switch {
	case id == idSetName:
		m.editingName = true
		m.usernameInput.SetValue(m.username)
		m.usernameInput.CursorEnd()
		m.usernameInput.Focus()
		cmds = append(cmds, textinput.Blink)
	case id == idSetTime:
		m.settings.ShowTime = !m.settings.ShowTime
		m.settings.save()
	case id == idSetBack:
		m.state = stateHome
	case strings.HasPrefix(id, idSetTheme):
		if i, err := strconv.Atoi(strings.TrimPrefix(id, idSetTheme)); err == nil && i >= 0 && i < len(themes) {
			m.settings.Theme = themes[i].name
			applyTheme(i)
			m.settings.save()
		}
	}
	return cmds
}

func (m *model) cycleTheme(dir int) {
	i := (themeIndex(m.settings.Theme) + dir + len(themes)) % len(themes)
	m.settings.Theme = themes[i].name
	applyTheme(i)
	m.settings.save()
}

// ---- sohbet ----

func (m *model) updateChatKey(k tea.KeyMsg) []tea.Cmd {
	var cmds []tea.Cmd

	// Onay bekleyen dosya teklifi
	if len(m.offers) > 0 {
		switch k.String() {
		case "y", "Y":
			m.answerOffer(true)
		case "n", "N":
			m.answerOffer(false)
		}
		return cmds
	}

	// Ayrılma onayı
	if m.confirmLeave {
		switch k.String() {
		case "y", "Y", "enter":
			cmds = append(cmds, m.leaveRoom()...)
		case "n", "N", "esc":
			m.confirmLeave = false
		}
		return cmds
	}

	switch k.Type {
	case tea.KeyEsc:
		m.confirmLeave = true
	case tea.KeyEnter:
		cmds = append(cmds, m.action(idChatSend)...)
	case tea.KeyTab:
		m.openPicker(&cmds)
	case tea.KeyF2:
		cmds = append(cmds, m.action(idChatUsers)...)
	default:
		// "yazıyor..." bilgisi (en fazla 2 saniyede bir)
		v := m.textarea.Value()
		if v != "" && !strings.HasPrefix(v, "/") && time.Since(m.lastTyping) > 2*time.Second {
			m.lastTyping = time.Now()
			for _, n := range m.nodes {
				n.Send(network.MsgTypeTyping, network.Typing{Sender: m.username})
			}
		}
	}
	return cmds
}

func (m *model) openPicker(cmds *[]tea.Cmd) {
	m.state = stateFilePicker
	*cmds = append(*cmds, m.picker.Init())
}

// runInput sohbet girişini (komut veya mesaj) işler.
func (m *model) runInput(v string) []tea.Cmd {
	var cmds []tea.Cmd
	switch {
	case v == "/help":
		m.addSystem("Komutlar: /file · /send <yol> · /users · /ara <kelime> · /clear · /leave · F2 (kişi paneli) · PgUp/PgDn (kaydır)")
	case v == "/clear":
		m.lines = nil
		m.refreshViewport()
	case v == "/leave":
		m.confirmLeave = true
	case v == "/users":
		m.addSystem("Odadaki kişiler: " + strings.Join(m.memberList(), ", "))
	case v == "/file":
		m.openPicker(&cmds)
	case strings.HasPrefix(v, "/send "):
		m.sendFile(cleanPath(strings.TrimPrefix(v, "/send ")))
	case strings.HasPrefix(v, "/ara "):
		m.search(strings.TrimSpace(strings.TrimPrefix(v, "/ara ")))
	case strings.HasPrefix(v, "/"):
		m.addSystem("Bilinmeyen komut. Komutlar için /help yazın.")
	default:
		if paths := parsePaths(v); paths != nil { // bırakılan dosya(lar) mesaj olarak gitmesin
			m.sendFiles(paths)
			return cmds
		}
		if len(m.nodes) == 0 {
			m.setToast("Bağlı kimse yok, mesaj gönderilemedi.", true)
			return cmds
		}
		txt := network.TextMessage{Sender: m.username, Text: v}
		for _, n := range m.nodes {
			n.Send(network.MsgTypeText, txt)
		}
		m.addChat(m.username, v)
		database.SaveMessage(m.roomName, m.username, v)
	}
	return cmds
}

func (m *model) search(q string) {
	if q == "" {
		m.addSystem("Kullanım: /ara <kelime>")
		return
	}
	res, err := database.SearchMessages(m.roomName, q, 20)
	if err != nil {
		m.addSystem(fmt.Sprintf("Arama başarısız: %v", err))
		return
	}
	m.lines = append(m.lines, chatLine{text: fmt.Sprintf("Arama: %q (%d sonuç)", q, len(res)), kind: lineDivider})
	for _, r := range res {
		m.lines = append(m.lines, chatLine{at: r.Timestamp, sender: r.Sender, text: r.Content, kind: lineChat})
	}
	m.lines = append(m.lines, chatLine{text: "Arama sonu", kind: lineDivider})
	m.refreshViewport()
}

// sendFiles sürükle-bırak ile gelen birden çok dosyayı sırayla teklif eder.
func (m *model) sendFiles(paths []string) {
	for _, p := range paths {
		m.sendFile(p)
	}
	if len(paths) > 1 {
		m.setToast(fmt.Sprintf("%d dosya için onay bekleniyor", len(paths)), false)
	}
}

func (m *model) sendFile(path string) {
	if len(m.nodes) == 0 {
		m.setToast("Bağlı kimse yok, dosya gönderilemez.", true)
		return
	}
	id, err := transfer.SendOffer(m.nodes, path)
	if err != nil {
		m.addSystem(fmt.Sprintf("Dosya gönderilemedi: %v", err))
		return
	}
	m.outgoing[id] = path
	m.addSystem(fmt.Sprintf("Dosya isteği gönderildi, onay bekleniyor: %s", filepath.Base(path)))
}

func (m *model) handlePacket(packet *network.Packet, from *network.Node) {
	switch packet.Type {
	case network.MsgTypeText:
		var txt network.TextMessage
		if json.Unmarshal(packet.Payload, &txt) != nil {
			return
		}
		if m.isHosting {
			txt.Sender = from.Name // gönderen adı host tarafından doğrulanır
		}
		m.addChat(txt.Sender, txt.Text)
		delete(m.typing, txt.Sender)
		database.SaveMessage(m.roomName, txt.Sender, txt.Text)

		// Grup sohbeti rölesi
		if m.isHosting {
			for _, n := range m.nodes {
				if n != from {
					n.Send(network.MsgTypeText, txt)
				}
			}
		}

	case network.MsgTypeTyping:
		var t network.Typing
		if json.Unmarshal(packet.Payload, &t) != nil {
			return
		}
		if m.isHosting {
			t.Sender = from.Name
			for _, n := range m.nodes {
				if n != from {
					n.Send(network.MsgTypeTyping, t)
				}
			}
		}
		m.typing[t.Sender] = time.Now()

	case network.MsgTypeUsers:
		if m.isHosting {
			return
		}
		var ul network.UserList
		if json.Unmarshal(packet.Payload, &ul) != nil {
			return
		}
		m.diffMembers(ul.Users)
		m.members = ul.Users

	case network.MsgTypeFileOffer:
		var offer network.FileOffer
		if json.Unmarshal(packet.Payload, &offer) != nil || len(m.offers) >= 5 {
			return
		}
		offer.FileName = transfer.SafeName(offer.FileName)
		m.offers = append(m.offers, incomingOffer{offer: offer, node: from})
		m.addSystem(fmt.Sprintf("%s bir dosya göndermek istiyor: %s", from.Name, offer.FileName))

	case network.MsgTypeFileAccept:
		var accept network.FileAccept
		if json.Unmarshal(packet.Payload, &accept) != nil {
			return
		}
		path, ok := m.outgoing[accept.FileID]
		if !ok {
			return
		}
		name := filepath.Base(path)
		label := name + " → " + from.Name
		key := "up:" + accept.FileID + ":" + from.Addr()
		m.transfers[key] = &activeTransfer{label: "↑ " + label, node: from}
		m.addSystem(fmt.Sprintf("%s dosyayı kabul etti. Gönderiliyor...", from.Name))
		go func() {
			err := transfer.SendChunked(from, accept.FileID, path, func(p float64) {
				if GlobalProgram != nil {
					GlobalProgram.Send(fileProgressMsg{key: key, percent: p})
				}
			})
			if GlobalProgram != nil {
				GlobalProgram.Send(fileFinishedMsg{key: key, label: label, err: err})
			}
		}()

	case network.MsgTypeFileReject:
		m.addSystem(from.Name + " dosyayı reddetti.")

	case network.MsgTypeFileChunk:
		var chunk network.FileChunk
		if json.Unmarshal(packet.Payload, &chunk) != nil {
			return
		}
		tr, ok := m.transfers["down:"+chunk.FileID]
		if !ok || tr.node != from { // yalnızca kabul ettiğimiz aktarımlar
			return
		}
		if tr.total > 0 && tr.got+int64(len(chunk.Data)) > tr.total {
			return
		}
		path := filepath.Join(Cfg.DownloadDir, transfer.SafeName(chunk.FileID)+".tmp")
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		f.Write(chunk.Data)
		f.Close()

		tr.got += int64(len(chunk.Data))
		if tr.total > 0 {
			tr.frac = float64(tr.got) / float64(tr.total)
		}

	case network.MsgTypeFileDone:
		var done network.FileDone
		if json.Unmarshal(packet.Payload, &done) != nil {
			return
		}
		key := "down:" + done.FileID
		tr, ok := m.transfers[key]
		if !ok || tr.node != from {
			return
		}
		delete(m.transfers, key)

		tmpPath := filepath.Join(Cfg.DownloadDir, transfer.SafeName(done.FileID)+".tmp")
		if sum, err := transfer.SHA256File(tmpPath); err != nil || (done.SHA256 != "" && sum != done.SHA256) {
			os.Remove(tmpPath)
			m.addSystem("Dosya bozuk geldi (özet uyuşmadı), kaydedilmedi: " + tr.name)
			return
		}
		finalPath := transfer.UniquePath(Cfg.DownloadDir, transfer.SafeName(tr.name))
		if err := os.Rename(tmpPath, finalPath); err != nil {
			m.addSystem(fmt.Sprintf("Dosya kaydedilemedi: %v", err))
			return
		}
		m.addSystem("Dosya kaydedildi (doğrulandı): " + finalPath)
	}
}

func (m *model) diffMembers(next []string) {
	old := map[string]bool{}
	for _, u := range m.members {
		old[u] = true
	}
	cur := map[string]bool{}
	for _, u := range next {
		cur[u] = true
		if !old[u] && len(m.members) > 0 && u != m.username {
			m.addSystem(u + " odaya katıldı.")
		}
	}
	for _, u := range m.members {
		if !cur[u] && u != m.username {
			m.addSystem(u + " odadan ayrıldı.")
		}
	}
}

// ---- mesaj satırları ----

func (m *model) loadHistory() {
	msgs, err := database.GetMessages(m.roomName, 50)
	if err != nil || len(msgs) == 0 {
		return
	}
	m.lines = append(m.lines, chatLine{text: "Geçmiş mesajlar", kind: lineDivider})
	for _, dbMsg := range msgs {
		m.lines = append(m.lines, chatLine{at: dbMsg.Timestamp, sender: dbMsg.Sender, text: dbMsg.Content, kind: lineChat})
	}
	m.lines = append(m.lines, chatLine{text: "Canlı sohbet", kind: lineDivider})
}

func (m *model) addChat(sender, text string) {
	m.lines = append(m.lines, chatLine{at: time.Now().Format("15:04"), sender: sender, text: text, kind: lineChat})
	m.refreshViewport()
}

func (m *model) addSystem(text string) {
	m.lines = append(m.lines, chatLine{at: time.Now().Format("15:04"), text: text, kind: lineSystem})
	m.refreshViewport()
}

// refreshViewport satırları genişliğe göre sararak viewport'a yazar.
func (m *model) refreshViewport() {
	w := m.viewport.Width
	if w < 12 {
		return
	}
	bodyW := w - 2 // sol renk çubuğu için
	wrap := lipgloss.NewStyle().Width(bodyW)
	ts := func(at string) string {
		if !m.settings.ShowTime || at == "" {
			return ""
		}
		return timeStyle.Render(at) + " "
	}
	out := make([]string, 0, len(m.lines))
	for _, l := range m.lines {
		switch l.kind {
		case lineDivider:
			out = append(out, divider(l.text, w))
		case lineSystem:
			out = append(out, wrap.Render(ts(l.at)+msgSystemStyle.Render("• "+l.text)))
		default:
			name := senderStyle(l.sender)
			if l.sender == m.username {
				name = msgUserStyle
			}
			body := wrap.Render(ts(l.at) + name.Render(l.sender) + "  " + l.text)
			rows := strings.Split(body, "\n")
			barStyle := lipgloss.NewStyle().Foreground(name.GetForeground())
			for i := range rows {
				rows[i] = barStyle.Render("▎") + " " + rows[i]
			}
			out = append(out, strings.Join(rows, "\n"))
		}
	}
	atBottom := m.viewport.AtBottom()
	m.viewport.SetContent(strings.Join(out, "\n"))
	if atBottom {
		m.viewport.GotoBottom()
	}
}

func divider(label string, w int) string {
	label = " " + label + " "
	side := (w - lipgloss.Width(label)) / 2
	if side < 1 {
		side = 1
	}
	line := strings.Repeat("─", side)
	return infoTextStyle.Render(line + label + line)
}

var GlobalProgram *tea.Program

func StartApp() error {
	os.MkdirAll(Cfg.DataDir, os.ModePerm)
	network.DiscoveryPort = Cfg.DiscoveryPort
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	GlobalProgram = p
	_, err := p.Run()
	return err
}

// startHostCmd odayı kurar ve dinlemeye başlar; gelen bağlantılar GlobalProgram.Send ile arayüze iletilir.
func startHostCmd(password, roomName, owner string, port int, stopCh <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		room, err := network.NewRoom(roomName, owner, password)
		if err != nil {
			return connectionErrorMsg{err: err}
		}
		listener, actualPort, err := room.Listen(port, func(node *network.Node) {
			if GlobalProgram != nil {
				GlobalProgram.Send(connectionEstablishedMsg{node: node})
			}
		})
		if err != nil {
			return connectionErrorMsg{err: err}
		}
		go network.StartBroadcasting(room.Discovery(actualPort), stopCh)
		return hostStartedMsg{listener: listener, port: actualPort}
	}
}

func connectCmd(address, password string, salt []byte, name string) tea.Cmd {
	return func() tea.Msg {
		node, ack, err := network.Connect(address, password, salt, name)
		if err != nil {
			return connectionErrorMsg{err: err}
		}
		return connectionEstablishedMsg{node: node, ack: ack}
	}
}
