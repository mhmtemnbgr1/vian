package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// InitDB veritabanını dir klasöründe açar (yoksa oluşturur).
func InitDB(dir string) error {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	dbPath := filepath.Join(dir, "vian.db")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	createTableQuery := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		room_name TEXT,
		sender TEXT,
		content TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_messages_room ON messages(room_name, id);`

	_, err = db.Exec(createTableQuery)
	return err
}

func SaveMessage(roomName, sender, content string) error {
	if db == nil {
		return fmt.Errorf("veritabanı başlatılmadı")
	}
	query := `INSERT INTO messages (room_name, sender, content) VALUES (?, ?, ?)`
	_, err := db.Exec(query, roomName, sender, content)
	return err
}

type Message struct {
	Sender    string
	Content   string
	Timestamp string
}

func GetMessages(roomName string, limit int) ([]Message, error) {
	return queryMessages(`SELECT sender, content, strftime('%d.%m %H:%M', timestamp, 'localtime')
		FROM messages WHERE room_name = ? ORDER BY id DESC LIMIT ?`, roomName, limit)
}

// SearchMessages odanın geçmişinde (büyük/küçük harf duyarsız) metin arar.
func SearchMessages(roomName, text string, limit int) ([]Message, error) {
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(text)
	return queryMessages(`SELECT sender, content, strftime('%d.%m %H:%M', timestamp, 'localtime')
		FROM messages WHERE room_name = ? AND content LIKE ? ESCAPE '\' ORDER BY id DESC LIMIT ?`,
		roomName, "%"+esc+"%", limit)
}

// queryMessages sonuçları kronolojik (eskiden yeniye) döndürür.
func queryMessages(query string, args ...any) ([]Message, error) {
	if db == nil {
		return nil, fmt.Errorf("veritabanı başlatılmadı")
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Sender, &m.Content, &m.Timestamp); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}

	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}
