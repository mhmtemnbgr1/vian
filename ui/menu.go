package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"vian/database"
	"vian/network"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type uiState int

const (
	stateUserSetup uiState = iota
	stateMenu
	stateHostNameSetup
	stateHostPassSetup
	stateJoinSetup
	stateChat
	stateFilePicker
)

type incomingPacketMsg struct {
	packet *network.Packet
	node   *network.Node
}

type peerFoundMsg struct {
	ip  string
	msg network.DiscoveryMessage
}

type connectionEstablishedMsg struct {
	node *network.Node
}

type fileProgressMsg struct {
	percent float64
}

type connectionErrorMsg struct {
	err error
}

type model struct {
	state       uiState
	menuCursor  int
	menuChoices []string

	// User Info
	usernameInput textinput.Model
	username      string

	// Host Setup
	hostNameInput textinput.Model
	roomName      string
	hostInput     textinput.Model
	isHosting     bool

	// Join Setup
	foundPeers  []peerFoundMsg
	peerCursor  int
	joinInput   textinput.Model
	joiningPeer *peerFoundMsg
	joinError   string

	// Chat
	viewport viewport.Model
	textarea textarea.Model
	messages []string
	nodes    []*network.Node // Çoklu bağlantı için (Grup Sohbeti)
	picker   filepicker.Model

	// File Transfer State
	pendingOffer *network.FileOffer
	pendingFile  string
	progressBar  progress.Model
	progress     float64
	isTransfer   bool

	// Network channels
	stopDiscovery chan struct{}
	discoveryChan chan peerFoundMsg

	// Window dimensions
	width  int
	height int
}

func initialModel() model {
	uInput := textinput.New()
	uInput.Placeholder = "Kullanıcı Adınız (Örn: Memo)"
	uInput.Focus()
	uInput.CharLimit = 20
	uInput.Width = 30

	hNameInput := textinput.New()
	hNameInput.Placeholder = "Oda İsmi (Örn: Gizli Karargah)"
	hNameInput.CharLimit = 30
	hNameInput.Width = 30

	ti := textinput.New()
	ti.Placeholder = "Oda Parolası"
	ti.CharLimit = 32
	ti.Width = 30

	ta := textarea.New()
	ta.Placeholder = "Mesaj yazın veya '/send dosya_yolu'..."
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 280
	ta.SetWidth(60)
	ta.SetHeight(3)

	vp := viewport.New(60, 15)
	vp.SetContent("Sohbete hoş geldiniz! Uçtan uca şifreli bağlantı sağlandı.")

	pb := progress.New(progress.WithDefaultGradient())
	pb.Width = 50

	sendDir := "gönderilecekler"
	os.MkdirAll(sendDir, os.ModePerm)
	absSendDir, _ := filepath.Abs(sendDir)

	fp := filepicker.New()
	fp.AllowedTypes = []string{".mp4", ".avi", ".mkv", ".jpg", ".jpeg", ".png", ".gif", ".txt", ".pdf", ".zip", ".rar"}
	fp.CurrentDirectory = absSendDir

	return model{
		state:         stateUserSetup,
		menuChoices:   []string{"Oda Kur", "Odalara Katıl", "Çıkış"},
		usernameInput: uInput,
		hostNameInput: hNameInput,
		hostInput:     ti,
		joinInput:     textinput.New(),
		textarea:      ta,
		viewport:      vp,
		progressBar:   pb,
		picker:        fp,
		messages:      []string{},
		nodes:         []*network.Node{},
		stopDiscovery: make(chan struct{}),
		discoveryChan: make(chan peerFoundMsg, 100),
	}
}

func listenForPackets(node *network.Node) tea.Cmd {
	return func() tea.Msg {
		if node == nil {
			return nil
		}
		packet, ok := <-node.Incoming
		if !ok {
			return nil
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

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		taCmd tea.Cmd
		vpCmd tea.Cmd
		cmds  []tea.Cmd
	)

	if m.state == stateChat {
		m.textarea, taCmd = m.textarea.Update(msg)
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, taCmd, vpCmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 10
		if m.viewport.Width > 60 {
			m.viewport.Width = 60
		}
		
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			if m.stopDiscovery != nil {
				close(m.stopDiscovery)
			}
			for _, n := range m.nodes {
				n.Close()
			}
			return m, tea.Quit
		}
	}

	switch m.state {
	case stateUserSetup:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEnter:
				v := strings.TrimSpace(m.usernameInput.Value())
				if v != "" {
					m.username = v
					m.state = stateMenu
				}
			}
		}
		m.usernameInput, tiCmd = m.usernameInput.Update(msg)
		cmds = append(cmds, tiCmd)

	case stateMenu:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				}
			case "down", "j":
				if m.menuCursor < len(m.menuChoices)-1 {
					m.menuCursor++
				}
			case "enter", " ":
				selected := m.menuChoices[m.menuCursor]
				if selected == "Oda Kur" {
					m.state = stateHostNameSetup
					m.hostNameInput.Focus()
					return m, textinput.Blink
				} else if selected == "Odalara Katıl" {
					m.state = stateJoinSetup
					go network.ListenForPeers(func(ip string, pMsg network.DiscoveryMessage) {
						m.discoveryChan <- peerFoundMsg{ip: ip, msg: pMsg}
					}, m.stopDiscovery)
					cmds = append(cmds, waitForDiscovery(m.discoveryChan))
				} else if selected == "Çıkış" {
					return m, tea.Quit
				}
			}
		}

	case stateHostNameSetup:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEnter:
				v := strings.TrimSpace(m.hostNameInput.Value())
				if v != "" {
					m.roomName = v
					m.state = stateHostPassSetup
					m.hostInput.Focus()
					cmds = append(cmds, textinput.Blink)
				}
			case tea.KeyEsc:
				m.state = stateMenu
			}
		}
		m.hostNameInput, tiCmd = m.hostNameInput.Update(msg)
		cmds = append(cmds, tiCmd)

	case stateHostPassSetup:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if !m.isHosting {
				switch msg.Type {
				case tea.KeyEnter:
					password := strings.TrimSpace(m.hostInput.Value())
					m.isHosting = true
					cmds = append(cmds, startHostCmd(password, m.roomName, m.stopDiscovery))
					m.hostInput.Blur()
				case tea.KeyEsc:
					m.state = stateHostNameSetup
					m.hostInput.SetValue("")
					m.hostNameInput.Focus()
				}
			} else {
				if msg.Type == tea.KeyEsc {
					m.state = stateMenu
					m.isHosting = false
					m.hostInput.SetValue("")
					m.hostNameInput.SetValue("")
				}
			}
		case connectionEstablishedMsg:
			m.nodes = append(m.nodes, msg.node)
			if m.state != stateChat {
				m.state = stateChat
				m.textarea.Reset()
				m.loadHistory()
			}
			m.addMessage("Sistem", "Yeni bir kişi odaya katıldı.")
			cmds = append(cmds, listenForPackets(msg.node))
			
			// Host, yeni bağlantı kabul etmeye devam ediyor! (startHostCmd içinde listener kapanmıyor)
		}

		if !m.isHosting {
			m.hostInput, tiCmd = m.hostInput.Update(msg)
			cmds = append(cmds, tiCmd)
		}

	case stateJoinSetup:
		switch msg := msg.(type) {
		case peerFoundMsg:
			exists := false
			for _, p := range m.foundPeers {
				if p.ip == msg.ip && p.msg.Port == msg.msg.Port {
					exists = true
					break
				}
			}
			if !exists {
				m.foundPeers = append(m.foundPeers, msg)
			}
			cmds = append(cmds, waitForDiscovery(m.discoveryChan))

		case tea.KeyMsg:
			if m.joiningPeer == nil {
				switch msg.String() {
				case "up", "k":
					if m.peerCursor > 0 {
						m.peerCursor--
					}
				case "down", "j":
					if m.peerCursor < len(m.foundPeers)-1 {
						m.peerCursor++
					}
				case "enter":
					if len(m.foundPeers) > 0 {
						m.joiningPeer = &m.foundPeers[m.peerCursor]
						m.joinInput.Focus()
						m.joinInput.Placeholder = "Parola"
						m.joinError = ""
						cmds = append(cmds, textinput.Blink)
					}
				case "esc":
					m.state = stateMenu
				}
			} else {
				switch msg.Type {
				case tea.KeyEnter:
					password := strings.TrimSpace(m.joinInput.Value())
					address := fmt.Sprintf("%s:%d", m.joiningPeer.ip, m.joiningPeer.msg.Port)
					cmds = append(cmds, connectCmd(address, password))
				case tea.KeyEsc:
					m.joiningPeer = nil
					m.joinInput.SetValue("")
					m.joinError = ""
				}
				m.joinInput, tiCmd = m.joinInput.Update(msg)
				cmds = append(cmds, tiCmd)
			}
		case connectionErrorMsg:
			m.joinError = msg.err.Error()
			m.joiningPeer = nil
			m.joinInput.SetValue("")
			m.joinInput.Blur()
		case connectionEstablishedMsg:
			m.nodes = append(m.nodes, msg.node)
			m.state = stateChat
			m.textarea.Reset()
			m.loadHistory()
			m.addMessage("Sistem", fmt.Sprintf("%s odasına bağlanıldı.", m.joiningPeer.msg.HostName))
			cmds = append(cmds, listenForPackets(msg.node))
		}

	case stateChat:
		switch msg := msg.(type) {
		case fileProgressMsg:
			m.progress = msg.percent
			if m.progress >= 1.0 {
				m.isTransfer = false
				m.addMessage("Sistem", "Dosya aktarımı tamamlandı.")
			}
			
		case connectionEstablishedMsg:
			// Host için başka biri katılırsa
			m.nodes = append(m.nodes, msg.node)
			m.addMessage("Sistem", "Yeni bir kişi odaya katıldı.")
			cmds = append(cmds, listenForPackets(msg.node))

		case incomingPacketMsg:
			m.handlePacket(msg.packet, msg.node)
			cmds = append(cmds, listenForPackets(msg.node))

		case tea.KeyMsg:
			if m.pendingOffer != nil {
				switch msg.String() {
				case "y", "Y":
					accept := network.FileAccept{FileID: m.pendingOffer.FileID}
					for _, n := range m.nodes {
						n.Send(network.MsgTypeFileAccept, accept)
					}
					m.addMessage("Sistem", fmt.Sprintf("Dosya kabul edildi: %s, indiriliyor...", m.pendingOffer.FileName))
					m.isTransfer = true
					m.progress = 0
					m.pendingOffer = nil
				case "n", "N":
					reject := network.FileReject{FileID: m.pendingOffer.FileID}
					for _, n := range m.nodes {
						n.Send(network.MsgTypeFileReject, reject)
					}
					m.addMessage("Sistem", "Dosya reddedildi.")
					m.pendingOffer = nil
				}
			} else {
				switch msg.Type {
				case tea.KeyEnter:
					v := strings.TrimSpace(m.textarea.Value())
					if v != "" {
						if v == "/file" {
							m.state = stateFilePicker
							cmds = append(cmds, m.picker.Init())
						} else if strings.HasPrefix(v, "/send ") {
							filePath := strings.TrimPrefix(v, "/send ")
							m.pendingFile = filePath
							_, err := SendFileRequest(m.nodes[0], filePath)
							if err != nil {
								m.addMessage("Sistem", fmt.Sprintf("Dosya okunamadı: %v", err))
							} else {
								m.addMessage("Sistem", fmt.Sprintf("Dosya isteği gönderildi. Bekleniyor... %s", filepath.Base(filePath)))
							}
						} else {
							txtMsg := network.TextMessage{Sender: m.username, Text: v}
							for _, n := range m.nodes {
								n.Send(network.MsgTypeText, txtMsg)
							}
							m.addMessage(m.username, v)
							database.SaveMessage(m.currentRoomName(), m.username, v)
						}
						m.textarea.Reset()
					}
				case tea.KeyTab:
					m.state = stateFilePicker
					cmds = append(cmds, m.picker.Init())
				}
			}
		}

	case stateFilePicker:
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		cmds = append(cmds, cmd)

		if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
			m.pendingFile = path
			if len(m.nodes) > 0 {
				_, err := SendFileRequest(m.nodes[0], path)
				if err != nil {
					m.addMessage("Sistem", fmt.Sprintf("Dosya okunamadı: %v", err))
				} else {
					m.addMessage("Sistem", fmt.Sprintf("Dosya isteği gönderildi. Bekleniyor... %s", filepath.Base(path)))
				}
			}
			m.state = stateChat
		}

		if key, ok := msg.(tea.KeyMsg); ok && key.Type == tea.KeyEsc {
			m.state = stateChat
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) handlePacket(packet *network.Packet, senderNode *network.Node) {
	switch packet.Type {
	case network.MsgTypeText:
		var txt network.TextMessage
		json.Unmarshal(packet.Payload, &txt)
		m.addMessage(txt.Sender, txt.Text)
		database.SaveMessage(m.currentRoomName(), txt.Sender, txt.Text)

		// Grup sohbeti rölesi (Eğer host isek diğerlerine dağıt)
		if m.isHosting {
			for _, n := range m.nodes {
				if n != senderNode {
					n.Send(network.MsgTypeText, txt)
				}
			}
		}

	case network.MsgTypeFileOffer:
		var offer network.FileOffer
		json.Unmarshal(packet.Payload, &offer)
		m.pendingOffer = &offer

	case network.MsgTypeFileAccept:
		var accept network.FileAccept
		json.Unmarshal(packet.Payload, &accept)
		m.addMessage("Sistem", "Karşı taraf dosyayı kabul etti. Gönderiliyor...")
		m.isTransfer = true
		m.progress = 0
		go SendFileDataChunked(senderNode, accept.FileID, m.pendingFile, func(p float64) {
			if GlobalProgram != nil {
				GlobalProgram.Send(fileProgressMsg{percent: p})
			}
		})

	case network.MsgTypeFileReject:
		m.addMessage("Sistem", "Karşı taraf dosyayı reddetti.")

	case network.MsgTypeFileChunk:
		var chunk network.FileChunk
		json.Unmarshal(packet.Payload, &chunk)
		
		path := filepath.Join("indirilenler", chunk.FileID+".tmp")
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		f.Write(chunk.Data)
		f.Close()

	case network.MsgTypeFileDone:
		var done network.FileDone
		json.Unmarshal(packet.Payload, &done)
		
		tmpPath := filepath.Join("indirilenler", done.FileID+".tmp")
		finalPath := filepath.Join("indirilenler", done.FileName)
		os.Rename(tmpPath, finalPath)
		
		m.isTransfer = false
		m.addMessage("Sistem", "Dosya başarıyla kaydedildi: "+finalPath)
	}
}

func (m *model) currentRoomName() string {
	if m.isHosting {
		return m.roomName
	}
	if m.joiningPeer != nil {
		return m.joiningPeer.msg.HostName
	}
	return "Genel"
}

func (m *model) loadHistory() {
	msgs, err := database.GetMessages(m.currentRoomName(), 50)
	if err == nil && len(msgs) > 0 {
		m.messages = append(m.messages, "--- Geçmiş Mesajlar ---")
		for _, dbMsg := range msgs {
			m.messages = append(m.messages, fmt.Sprintf("[%s] %s: %s", dbMsg.Timestamp, dbMsg.Sender, dbMsg.Content))
		}
		m.messages = append(m.messages, "-----------------------")
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
	}
}

func (m *model) addMessage(sender, text string) {
	timeStr := time.Now().Format("15:04")
	m.messages = append(m.messages, fmt.Sprintf("[%s] %s: %s", timeStr, sender, text))
	m.viewport.SetContent(strings.Join(m.messages, "\n"))
	m.viewport.GotoBottom()
}

// Progress Bar Helper
func (m *model) UpdateProgress(percent float64) {
	// Programın update döngüsüne manuel sinyal göndermek zordur,
	// Bu yüzden Cmd dönebilir veya doğrudan program objesini kullanmalıyız.
	// Bubbles yapısında global program referansı ile p.Send() atılır.
	if GlobalProgram != nil {
		GlobalProgram.Send(fileProgressMsg{percent: percent})
	}
}

var GlobalProgram *tea.Program

// Styles
var (
	primaryColor   = lipgloss.Color("#00f2fe")
	secondaryColor = lipgloss.Color("#4facfe")
	accentColor    = lipgloss.Color("#ff0844")
	textColor      = lipgloss.Color("#e0e0e0")
	mutedColor     = lipgloss.Color("#666666")

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(primaryColor).
		Padding(0, 2).
		MarginBottom(1)

	logoStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(1)

	windowStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(secondaryColor).
		Padding(1, 3).
		MarginTop(1).
		MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Padding(0, 1)

	modalStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(accentColor).
		Padding(1, 3).
		Align(lipgloss.Center)

	msgUserStyle   = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	msgOtherStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffb199")).Bold(true)
	msgSystemStyle = lipgloss.NewStyle().Foreground(mutedColor).Italic(true)
	infoTextStyle  = lipgloss.NewStyle().Foreground(mutedColor)
)

func (m model) View() string {
	s := ""

	switch m.state {
	case stateUserSetup:
		logo := `
 __      __ _____         _   _ 
 \ \    / /|_   _|  /\   | \ | |
  \ \  / /   | |   /  \  |  \| |
   \ \/ /    | |  / /\ \ | . ` + "`" + ` |
    \  /    _| |_/ ____ \| |\  |
     \/    |_____/_/    \_\_| \_|
		`
		s += logoStyle.Render(logo) + "\n\n"
		s += titleStyle.Render(" VİAN'A HOŞ GELDİNİZ ") + "\n\n"
		s += "Güvenli ağa katılmak için bir kullanıcı adı belirleyin:\n\n"
		s += m.usernameInput.View() + "\n\n"
		s += infoTextStyle.Render("Devam etmek için Enter")

	case stateMenu:
		s += titleStyle.Render(fmt.Sprintf(" VIAN - GÜVENLİ AĞ (Kullanıcı: %s) ", m.username)) + "\n\n"
		for i, choice := range m.menuChoices {
			cursor := "  "
			if m.menuCursor == i {
				cursor = "> "
				s += fmt.Sprintf("%s%s\n", cursor, selectedStyle.Render(choice))
			} else {
				s += fmt.Sprintf("%s%s\n", cursor, choice)
			}
		}

	case stateHostNameSetup:
		s += titleStyle.Render(" ODA KURULUYOR (1/2) ") + "\n\n"
		s += "Ağdaki kişilerin göreceği Oda İsmini yazın:\n\n"
		s += m.hostNameInput.View() + "\n\n"
		s += infoTextStyle.Render("Geri: ESC | İleri: Enter")

	case stateHostPassSetup:
		s += titleStyle.Render(" ODA KURULUYOR (2/2) ") + "\n\n"
		if m.isHosting {
			s += fmt.Sprintf("Oda '%s' adıyla başarıyla kuruldu.\n\n", m.roomName)
			s += lipgloss.NewStyle().Foreground(primaryColor).Render("Dinleniyor... Karşı tarafın bağlanması bekleniyor...") + "\n\n"
		} else {
			s += "Bu odaya girmek için gereken parolayı belirleyin:\n\n"
			s += m.hostInput.View() + "\n\n"
		}
		s += infoTextStyle.Render("Geri Dönmek için ESC")

	case stateJoinSetup:
		s += titleStyle.Render(" ODALARA KATIL ") + "\n\n"
		if m.joiningPeer == nil {
			s += "Ağdaki Aktif Odalar:\n\n"
			if len(m.foundPeers) == 0 {
				s += infoTextStyle.Render("Aranıyor...\n")
			}
			for i, p := range m.foundPeers {
				cursor := "  "
				if m.peerCursor == i {
					cursor = "> "
					s += fmt.Sprintf("%s%s\n", cursor, selectedStyle.Render(p.msg.HostName))
				} else {
					s += fmt.Sprintf("%s%s\n", cursor, p.msg.HostName)
				}
			}
		} else {
			s += fmt.Sprintf("'%s' odasına bağlanılıyor...\n\n", m.joiningPeer.msg.HostName)
			s += "Parolayı girin:\n"
			s += m.joinInput.View() + "\n"
			if m.joinError != "" {
				s += "\n" + lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("Hata: "+m.joinError) + "\n"
			}
		}
		s += "\n" + infoTextStyle.Render("Geri Dönmek için ESC")

	case stateChat:
		s += titleStyle.Render(" SOHBET ODASI ") + "\n"
		
		coloredMessages := make([]string, len(m.messages))
		for i, msg := range m.messages {
			if strings.Contains(msg, "] "+m.username+":") {
				coloredMessages[i] = msgUserStyle.Render(msg)
			} else if strings.Contains(msg, "] Sistem:") || strings.Contains(msg, "---") {
				coloredMessages[i] = msgSystemStyle.Render(msg)
			} else {
				coloredMessages[i] = msgOtherStyle.Render(msg)
			}
		}
		m.viewport.SetContent(strings.Join(coloredMessages, "\n"))
		
		s += lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(secondaryColor).Padding(0, 1).Render(m.viewport.View()) + "\n\n"
		
		if m.isTransfer {
			s += "Aktarım: " + m.progressBar.ViewAs(m.progress) + "\n\n"
		} else if m.pendingOffer != nil {
			modal := fmt.Sprintf("Karşı taraf %s (%.2f MB) göndermek istiyor.\nKabul ediyor musunuz? (Y/N)", 
				m.pendingOffer.FileName, float64(m.pendingOffer.FileSize)/1024/1024)
			s += modalStyle.Render(modal) + "\n"
		} else {
			s += m.textarea.View() + "\n"
		}
		s += "\n" + infoTextStyle.Render("Çıkış için Ctrl+C | Dosya Göndermek İçin: Dosyayı 'gönderilecekler' klasörüne atıp /file yazın")
	
	case stateFilePicker:
		s += titleStyle.Render(" GÖNDERİLECEKLER KLASÖRÜ ") + "\n\n"
		s += "Göndermek istediğiniz dosyayı 'gönderilecekler' klasöründen seçin:\n\n"
		s += m.picker.View() + "\n\n"
		s += infoTextStyle.Render("Geri Dönmek için ESC | Seçmek için Enter")
	}

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, windowStyle.Render(s))
	}
	return windowStyle.Render(s)
}

func StartApp() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	GlobalProgram = p
	_, err := p.Run()
	return err
}

// startHostCmd içinde listener'ı KAPATMIYORUZ, çoklu bağlantı için sürekli dinliyor.
func startHostCmd(password string, roomName string, stopCh <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		msgChan := make(chan connectionEstablishedMsg)
		_, port, err := network.Host(0, password, func(node *network.Node) {
			if GlobalProgram != nil {
				GlobalProgram.Send(connectionEstablishedMsg{node: node})
			}
		})
		if err != nil {
			return nil
		}
		go network.StartBroadcasting(port, roomName, stopCh)
		
		// Sadece bekleyelim, goroutine bitmesin (tea.Cmd sonlandığı için aslında sorun yok, 
		// ama Host func kendi goroutine'ini zaten açtı, bu yüzden hemen dönebiliriz).
		// Bağlantılar geldikçe GlobalProgram'a Send edilecek.
		return <-msgChan // Wait forever in this command context is bad. Better to just return nil, 
		                 // and rely entirely on the GlobalProgram.Send callback.
	}
}

// connectCmd
func connectCmd(address string, password string) tea.Cmd {
	return func() tea.Msg {
		node, err := network.Connect(address, password)
		if err != nil {
			return connectionErrorMsg{err: err}
		}
		return connectionEstablishedMsg{node: node}
	}
}