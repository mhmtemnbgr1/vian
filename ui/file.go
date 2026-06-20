package ui

import (
	"io"
	"os"
	"path/filepath"
	"vian/network"

	"github.com/google/uuid"
)

const DownloadDir = "indirilenler"
const UploadDir = "gönderilecekler"

func init() {
	os.MkdirAll(DownloadDir, os.ModePerm)
	os.MkdirAll(UploadDir, os.ModePerm)
}

func SendFileRequest(node *network.Node, filePath string) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	fileID := uuid.New().String()
	offer := network.FileOffer{
		FileID:   fileID,
		FileName: filepath.Base(filePath),
		FileSize: fileInfo.Size(),
	}

	err = node.Send(network.MsgTypeFileOffer, offer)
	return fileID, err
}

func SendFileDataChunked(node *network.Node, fileID string, filePath string, updateProgress func(float64)) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	stat, _ := file.Stat()
	totalSize := stat.Size()

	buf := make([]byte, 64*1024) // 64KB chunks
	var sent int64 = 0

	for {
		n, err := file.Read(buf)
		if n > 0 {
			chunk := network.FileChunk{
				FileID: fileID,
				Data:   buf[:n],
			}
			node.Send(network.MsgTypeFileChunk, chunk)
			sent += int64(n)

			if updateProgress != nil {
				updateProgress(float64(sent) / float64(totalSize))
			}
		}
		if err == io.EOF {
			break
		}
	}

	done := network.FileDone{
		FileID:   fileID,
		FileName: filepath.Base(filePath),
	}
	node.Send(network.MsgTypeFileDone, done)
}

// SaveFileData artık kullanılmıyor, yerine tmp ve append mantığı geldi (menu.go içinde).
