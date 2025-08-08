package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB // Kapital → diekspor

func InitDB() {
	var err error
	DB, err = sql.Open("sqlite3", "./filemeta.db")
	if err != nil {
		panic(err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS uploads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		filename TEXT,
		username TEXT,
		uploaded_at DATETIME
	);
	`
	_, err = DB.Exec(createTable)
	if err != nil {
		panic(err)
	}
}
