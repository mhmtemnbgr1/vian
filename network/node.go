package network

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	maxFrameSize     = 8 << 20 // bellek tüketimini sınırlamak için
	handshakeTimeout = 10 * time.Second
	handshakeMagic   = "VIAN"
)

type Node struct {
	Conn      net.Conn
	Name      string // karşı tarafın kullanıcı adı
	Incoming  chan *Packet
	IsHost    bool
	keyMu     sync.RWMutex
	key       []byte
	writeLock sync.Mutex
}

func newNode(conn net.Conn, key []byte, isHost bool) *Node {
	return &Node{
		Conn:     conn,
		key:      key,
		Incoming: make(chan *Packet, 100),
		IsHost:   isHost,
	}
}

func (n *Node) getKey() []byte {
	n.keyMu.RLock()
	defer n.keyMu.RUnlock()
	return n.key
}

func (n *Node) setKey(k []byte) {
	n.keyMu.Lock()
	n.key = k
	n.keyMu.Unlock()
}

// Start okuma döngüsünü başlatır (el sıkışma bittikten sonra çağrılır).
func (n *Node) Start() { go n.readLoop() }

func (n *Node) Close() error {
	return n.Conn.Close()
}

// Addr karşı tarafın adresini döndürür.
func (n *Node) Addr() string {
	return n.Conn.RemoteAddr().String()
}

func (n *Node) Send(msgType MsgType, data interface{}) error {
	packetBytes, err := Pack(msgType, data)
	if err != nil {
		return err
	}

	encrypted, err := Encrypt(packetBytes, n.getKey())
	if err != nil {
		return err
	}

	frame := make([]byte, 4+len(encrypted))
	binary.BigEndian.PutUint32(frame, uint32(len(encrypted)))
	copy(frame[4:], encrypted)

	n.writeLock.Lock()
	defer n.writeLock.Unlock()
	_, err = n.Conn.Write(frame)
	return err
}

func readPacket(conn net.Conn, key []byte) (*Packet, error) {
	lengthPrefix := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthPrefix); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthPrefix)
	if length == 0 || length > maxFrameSize {
		return nil, errors.New("geçersiz paket boyutu")
	}
	encrypted := make([]byte, length)
	if _, err := io.ReadFull(conn, encrypted); err != nil {
		return nil, err
	}
	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		return nil, err
	}
	return Unpack(decrypted)
}

func (n *Node) readLoop() {
	defer close(n.Incoming)
	for {
		packet, err := readPacket(n.Conn, n.getKey())
		if err != nil {
			n.Conn.Close()
			return
		}
		n.Incoming <- packet
	}
}

// SanitizeName kullanıcı adındaki kontrol karakterlerini atar ve uzunluğu sınırlar.
func SanitizeName(name string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if r := []rune(name); len(r) > 20 {
		name = string(r[:20])
	}
	if name == "" {
		return "Misafir"
	}
	return name
}

// Room host tarafındaki oda bilgisi ve anahtarlarını tutar.
type Room struct {
	Name       string
	Owner      string
	Salt       []byte
	pwKey      []byte
	sessionKey []byte
}

func NewRoom(name, owner, password string) (*Room, error) {
	salt, err := NewSalt()
	if err != nil {
		return nil, err
	}
	pw, err := DeriveKey(password, salt)
	if err != nil {
		return nil, err
	}
	session, err := RandomBytes(32)
	if err != nil {
		return nil, err
	}
	return &Room{Name: name, Owner: owner, Salt: salt, pwKey: pw, sessionKey: session}, nil
}

// Discovery yayınlanacak oda duyurusunu üretir.
func (r *Room) Discovery(tcpPort int) DiscoveryMessage {
	return DiscoveryMessage{ServiceName: "vian-chat", Port: tcpPort, HostName: r.Name, Owner: r.Owner, Salt: r.Salt}
}

// Listen verilen portta (0 = rastgele) bağlantı kabul eder.
func (r *Room) Listen(port int, onConnect func(*Node)) (net.Listener, int, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, 0, err
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // Listener kapandıysa çık
			}
			go r.accept(conn, onConnect)
		}
	}()
	return listener, actualPort, nil
}

func (r *Room) accept(conn net.Conn, onConnect func(*Node)) {
	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	packet, err := readPacket(conn, r.pwKey) // parola yanlışsa çözülemez
	if err != nil || packet.Type != MsgTypeHandshake {
		conn.Close()
		return
	}
	var hs Handshake
	if json.Unmarshal(packet.Payload, &hs) != nil || hs.Magic != handshakeMagic {
		conn.Close()
		return
	}

	n := newNode(conn, r.pwKey, true)
	n.Name = SanitizeName(hs.Name)
	ack := HandshakeAck{Status: "OK", Room: r.Name, Owner: r.Owner, SessionKey: r.sessionKey}
	if err := n.Send(MsgTypeHandshakeAck, ack); err != nil {
		conn.Close()
		return
	}
	n.setKey(r.sessionKey)
	conn.SetDeadline(time.Time{})
	n.Start()
	onConnect(n)
}

// Connect bir odaya bağlanır; tuz, host'un duyurusundan gelir.
func Connect(address, password string, salt []byte, name string) (*Node, *HandshakeAck, error) {
	key, err := DeriveKey(password, salt)
	if err != nil {
		return nil, nil, err
	}
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, nil, err
	}
	conn.SetDeadline(time.Now().Add(handshakeTimeout))

	n := newNode(conn, key, false)
	if err := n.Send(MsgTypeHandshake, Handshake{Magic: handshakeMagic, Name: name}); err != nil {
		conn.Close()
		return nil, nil, err
	}

	packet, err := readPacket(conn, key)
	if err != nil {
		conn.Close()
		return nil, nil, errors.New("bağlantı reddedildi: parola hatalı olabilir")
	}
	var ack HandshakeAck
	if packet.Type != MsgTypeHandshakeAck || json.Unmarshal(packet.Payload, &ack) != nil || len(ack.SessionKey) != 32 {
		conn.Close()
		return nil, nil, errors.New("beklenmeyen sunucu yanıtı")
	}

	n.Name = ack.Owner
	n.setKey(ack.SessionKey)
	conn.SetDeadline(time.Time{})
	n.Start()
	return n, &ack, nil
}
