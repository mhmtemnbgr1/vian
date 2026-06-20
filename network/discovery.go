package network

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const DiscoveryPort = 8888

type DiscoveryMessage struct {
	ServiceName string `json:"service_name"`
	Port        int    `json:"port"`
	HostName    string `json:"host_name"`
}

// StartBroadcasting broadcasts the server's presence every 2 seconds
func StartBroadcasting(tcpPort int, hostName string, stopCh <-chan struct{}) {
	addr := &net.UDPAddr{
		IP:   net.IPv4bcast, // 255.255.255.255
		Port: DiscoveryPort,
	}
	
	// Create UDP connection
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		fmt.Printf("Broadcast hatası: %v\n", err)
		return
	}
	defer conn.Close()

	msg := DiscoveryMessage{
		ServiceName: "vian-chat",
		Port:        tcpPort,
		HostName:    hostName,
	}
	data, _ := json.Marshal(msg)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			conn.Write(data)
		}
	}
}

// ListenForPeers listens for broadcast messages from other peers
func ListenForPeers(peerFound func(ip string, msg DiscoveryMessage), stopCh <-chan struct{}) {
	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: DiscoveryPort,
	}
	
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("Dinleme hatası: %v\n", err)
		return
	}
	defer conn.Close()

	go func() {
		<-stopCh
		conn.Close()
	}()

	buf := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return // Kapandıysa veya hata varsa çık
		}

		var msg DiscoveryMessage
		if err := json.Unmarshal(buf[:n], &msg); err == nil && msg.ServiceName == "vian-chat" {
			peerFound(remoteAddr.IP.String(), msg)
		}
	}
}
