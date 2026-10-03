package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// DiscoveryPort UDP keşif portu; bayrakla değiştirilebilir.
var DiscoveryPort = 8888

type DiscoveryMessage struct {
	ServiceName string `json:"service_name"`
	Port        int    `json:"port"`
	HostName    string `json:"host_name"` // oda adı
	Owner       string `json:"owner"`     // odayı kuran kullanıcı
	Salt        []byte `json:"salt"`      // parola türetme tuzu
}

// StartBroadcasting odanın varlığını 2 saniyede bir yayınlar.
func StartBroadcasting(msg DiscoveryMessage, stopCh <-chan struct{}) error {
	addr := &net.UDPAddr{
		IP:   net.IPv4bcast, // 255.255.255.255
		Port: DiscoveryPort,
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	data, _ := json.Marshal(msg)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return nil
		case <-ticker.C:
			conn.Write(data)
		}
	}
}

// ListenForPeers ağdaki oda duyurularını dinler.
func ListenForPeers(peerFound func(ip string, msg DiscoveryMessage), stopCh <-chan struct{}) error {
	lc := net.ListenConfig{Control: reuseAddr}
	pc, err := lc.ListenPacket(context.Background(), "udp4", fmt.Sprintf(":%d", DiscoveryPort))
	if err != nil {
		return err
	}
	conn := pc.(*net.UDPConn)
	defer conn.Close()

	go func() {
		<-stopCh
		conn.Close()
	}()

	buf := make([]byte, 2048)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return nil // kapandı
		}

		var msg DiscoveryMessage
		if err := json.Unmarshal(buf[:n], &msg); err == nil && msg.ServiceName == "vian-chat" && len(msg.Salt) > 0 {
			peerFound(remoteAddr.IP.String(), msg)
		}
	}
}
