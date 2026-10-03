package network

import (
	"fmt"
	"testing"
	"time"
)

func TestEncryptDecrypt(t *testing.T) {
	key, err := DeriveKey("parola", []byte("0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := Encrypt([]byte("merhaba"), key)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decrypt(enc, key)
	if err != nil || string(dec) != "merhaba" {
		t.Fatalf("beklenen 'merhaba', gelen %q (%v)", dec, err)
	}

	other, _ := DeriveKey("baska", []byte("0123456789abcdef"))
	if _, err := Decrypt(enc, other); err == nil {
		t.Fatal("yanlış anahtarla çözme başarılı olmamalı")
	}
}

func TestDeriveKeyUsesSalt(t *testing.T) {
	a, _ := DeriveKey("p", []byte("salt-a-salt-a-sa"))
	b, _ := DeriveKey("p", []byte("salt-b-salt-b-sa"))
	if string(a) == string(b) {
		t.Fatal("farklı tuzlar farklı anahtar üretmeli")
	}
}

func startRoom(t *testing.T, password string) (*Room, int, chan *Node) {
	t.Helper()
	room, err := NewRoom("oda", "host", password)
	if err != nil {
		t.Fatal(err)
	}
	joined := make(chan *Node, 4)
	l, port, err := room.Listen(0, func(n *Node) { joined <- n })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return room, port, joined
}

func TestHandshakeAndMessage(t *testing.T) {
	room, port, joined := startRoom(t, "gizli")
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	client, ack, err := Connect(addr, "gizli", room.Salt, "Ali")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if ack.Room != "oda" || ack.Owner != "host" || client.Name != "host" {
		t.Fatalf("beklenmeyen ack: %+v", ack)
	}

	var server *Node
	select {
	case server = <-joined:
	case <-time.After(3 * time.Second):
		t.Fatal("host bağlantıyı görmedi")
	}
	defer server.Close()
	if server.Name != "Ali" {
		t.Fatalf("host kullanıcı adı = %q", server.Name)
	}

	// Oturum anahtarına geçildikten sonra iki yönde mesaj akmalı.
	if err := client.Send(MsgTypeText, TextMessage{Sender: "Ali", Text: "selam"}); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-server.Incoming:
		if p.Type != MsgTypeText {
			t.Fatalf("tür = %s", p.Type)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("host mesajı almadı")
	}
	if err := server.Send(MsgTypeUsers, UserList{Users: []string{"host", "Ali"}}); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-client.Incoming:
		if p.Type != MsgTypeUsers {
			t.Fatalf("tür = %s", p.Type)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("istemci mesajı almadı")
	}
}

func TestWrongPasswordRejected(t *testing.T) {
	room, port, joined := startRoom(t, "gizli")
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	if _, _, err := Connect(addr, "yanlis", room.Salt, "Mallory"); err == nil {
		t.Fatal("yanlış parola reddedilmeli")
	}
	select {
	case <-joined:
		t.Fatal("yanlış parolayla host'a bağlantı bildirilmemeli")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestSanitizeName(t *testing.T) {
	if got := SanitizeName("  \x1b[31mAli\n "); got != "[31mAli" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeName("   "); got != "Misafir" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeName("123456789012345678901234"); len([]rune(got)) != 20 {
		t.Fatalf("uzunluk %d", len([]rune(got)))
	}
}
