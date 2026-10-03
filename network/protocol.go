package network

import (
	"encoding/json"
)

type MsgType string

const (
	MsgTypeText         MsgType = "TEXT"
	MsgTypeTyping       MsgType = "TYPING"
	MsgTypeUsers        MsgType = "USERS"
	MsgTypeFileOffer    MsgType = "FILE_OFFER"
	MsgTypeFileAccept   MsgType = "FILE_ACCEPT"
	MsgTypeFileReject   MsgType = "FILE_REJECT"
	MsgTypeFileChunk    MsgType = "FILE_CHUNK"
	MsgTypeFileDone     MsgType = "FILE_DONE"
	MsgTypeHandshake    MsgType = "HANDSHAKE"
	MsgTypeHandshakeAck MsgType = "HANDSHAKE_ACK"
)

type Packet struct {
	Type    MsgType `json:"type"`
	Payload []byte  `json:"payload"`
}

type TextMessage struct {
	Sender string `json:"sender"`
	Text   string `json:"text"`
}

type Typing struct {
	Sender string `json:"sender"`
}

// UserList odadaki kişilerin güncel listesidir (host yayınlar).
type UserList struct {
	Users []string `json:"users"`
}

type Handshake struct {
	Magic string `json:"magic"`
	Name  string `json:"name"`
}

// HandshakeAck parola doğrulandıktan sonra host'un oda bilgisini ve
// oturum anahtarını iletir. Bu paket parola anahtarıyla şifrelenir; sonraki
// tüm trafik rastgele oturum anahtarıyla şifrelenir.
type HandshakeAck struct {
	Status     string `json:"status"`
	Room       string `json:"room"`
	Owner      string `json:"owner"`
	SessionKey []byte `json:"session_key"`
}

type FileOffer struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
}

type FileAccept struct {
	FileID string `json:"file_id"`
}

type FileReject struct {
	FileID string `json:"file_id"`
}

type FileChunk struct {
	FileID string `json:"file_id"`
	Data   []byte `json:"data"`
}

type FileDone struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	SHA256   string `json:"sha256"`
}

func Pack(msgType MsgType, data interface{}) ([]byte, error) {
	payloadBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	p := Packet{
		Type:    msgType,
		Payload: payloadBytes,
	}
	return json.Marshal(p)
}

func Unpack(data []byte) (*Packet, error) {
	var p Packet
	err := json.Unmarshal(data, &p)
	return &p, err
}
