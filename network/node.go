package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

type Node struct {
	Conn      net.Conn
	Key       []byte
	Incoming  chan *Packet
	IsHost    bool
	writeLock sync.Mutex
}

func NewNode(conn net.Conn, password string, isHost bool) *Node {
	n := &Node{
		Conn:     conn,
		Key:      DeriveKey(password),
		Incoming: make(chan *Packet, 100),
		IsHost:   isHost,
	}
	go n.readLoop()
	return n
}

func Host(port int, password string, onConnect func(*Node)) (net.Listener, int, error) {
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
			node := NewNode(conn, password, true)
			go func(n *Node) {
				packet, ok := <-n.Incoming
				if !ok {
					return
				}
				if packet.Type == MsgTypeHandshake {
					n.Send(MsgTypeHandshakeAck, HandshakeAck{Status: "OK"})
					onConnect(n)
				} else {
					n.Close()
				}
			}(node)
		}
	}()

	return listener, actualPort, nil
}

func Connect(address string, password string) (*Node, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	n := NewNode(conn, password, false)
	
	err = n.Send(MsgTypeHandshake, Handshake{Magic: "VIAN"})
	if err != nil {
		n.Close()
		return nil, err
	}

	packet, ok := <-n.Incoming
	if !ok {
		return nil, fmt.Errorf("bağlantı reddedildi: parola hatalı")
	}
	if packet.Type != MsgTypeHandshakeAck {
		n.Close()
		return nil, fmt.Errorf("beklenmeyen sunucu yanıtı")
	}

	return n, nil
}

func (n *Node) Close() error {
	return n.Conn.Close()
}

func (n *Node) Send(msgType MsgType, data interface{}) error {
	packetBytes, err := Pack(msgType, data)
	if err != nil {
		return err
	}

	encrypted, err := Encrypt(packetBytes, n.Key)
	if err != nil {
		return err
	}

	lengthPrefix := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthPrefix, uint32(len(encrypted)))

	n.writeLock.Lock()
	defer n.writeLock.Unlock()

	if _, err := n.Conn.Write(lengthPrefix); err != nil {
		return err
	}
	if _, err := n.Conn.Write(encrypted); err != nil {
		return err
	}

	return nil
}

func (n *Node) readLoop() {
	for {
		lengthPrefix := make([]byte, 4)
		if _, err := io.ReadFull(n.Conn, lengthPrefix); err != nil {
			close(n.Incoming)
			return
		}

		length := binary.BigEndian.Uint32(lengthPrefix)
		
		encryptedData := make([]byte, length)
		if _, err := io.ReadFull(n.Conn, encryptedData); err != nil {
			close(n.Incoming)
			return
		}

		decrypted, err := Decrypt(encryptedData, n.Key)
		if err != nil {
			n.Conn.Close()
			close(n.Incoming)
			return
		}

		packet, err := Unpack(decrypted)
		if err != nil {
			continue
		}

		n.Incoming <- packet
	}
}
