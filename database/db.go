package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB() error {
	dir := "veri"
	os.MkdirAll(dir, os.ModePerm)
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
	);`

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
	if db == nil {
		return nil, fmt.Errorf("veritabanı başlatılmadı")
	}

	query := `SELECT sender, content, strftime('%H:%M', timestamp, 'localtime') as ts 
			  FROM messages WHERE room_name = ? ORDER BY id DESC LIMIT ?`

	rows, err := db.Query(query, roomName, limit)
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
	
	// Eski mesajları kronolojik sıralamak için listeyi tersine çeviriyoruz
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}
