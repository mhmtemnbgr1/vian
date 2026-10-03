// Package transfer terminal ve web arayüzünün ortak kullandığı dosya aktarım yardımcılarını içerir.
package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"vian/network"

	"github.com/google/uuid"
)

// SendOffer dosya teklifini tüm bağlantılara iletir ve teklif kimliğini döndürür.
func SendOffer(nodes []*network.Node, filePath string) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}
	if fileInfo.IsDir() {
		return "", fmt.Errorf("klasör gönderilemez")
	}

	fileID := uuid.New().String()
	offer := network.FileOffer{
		FileID:   fileID,
		FileName: filepath.Base(filePath),
		FileSize: fileInfo.Size(),
	}
	for _, n := range nodes {
		if err := n.Send(network.MsgTypeFileOffer, offer); err != nil {
			return "", err
		}
	}
	return fileID, nil
}

// SendChunked dosyayı 64KB parçalar halinde gönderir ve sonunda SHA-256 özetini iletir.
func SendChunked(node *network.Node, fileID string, filePath string, updateProgress func(float64)) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	totalSize := stat.Size()

	h := sha256.New()
	buf := make([]byte, 64*1024)
	var sent int64

	for {
		n, rerr := file.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
			chunk := network.FileChunk{FileID: fileID, Data: buf[:n]}
			if err := node.Send(network.MsgTypeFileChunk, chunk); err != nil {
				return err
			}
			sent += int64(n)
			if updateProgress != nil && totalSize > 0 {
				updateProgress(float64(sent) / float64(totalSize))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}

	return node.Send(network.MsgTypeFileDone, network.FileDone{
		FileID:   fileID,
		FileName: filepath.Base(filePath),
		SHA256:   hex.EncodeToString(h.Sum(nil)),
	})
}

// SHA256File diskteki dosyanın SHA-256 özetini hesaplar.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// UniquePath aynı adlı dosya varsa "ad (1).uzantı" biçiminde yeni bir yol üretir.
func UniquePath(dir, name string) string {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
}

// SafeName dışarıdan gelen dosya adından dizin bileşenlerini atar.
func SafeName(name string) string {
	name = strings.ReplaceAll(name, `\`, "/") // her iki ayraç da dizin kaçışıdır
	n := filepath.Base(filepath.Clean(name))
	if n == "." || n == string(filepath.Separator) || n == ".." || n == "/" {
		return "dosya"
	}
	return n
}
