package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // Statt go-sqlite3
)

func main() {
	// Datenbank öffnen
	db, err := sql.Open("sqlite", "taskTracker.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	fmt.Println("Datenbank geöffnet!")
	createTable(db)
	timer(db) // db als Parameter übergeben
}

func timer(db *sql.DB) {
	var (
		start time.Time
		end   time.Time
		input string
	)

	fmt.Println("type start to start the timer")

	for input != "start" {
		fmt.Scanln(&input)
		start = time.Now()

	}

	fmt.Println("timer started, type stop to stop the timer")
	for input != "stop" {
		fmt.Scanln(&input)
		end = time.Now()
	}

	fmt.Println("timer stopped")
	var title string
	for title == "" {
		fmt.Print("Titel eingeben: ")
		fmt.Scanln(&title)
		if title == "" {
			fmt.Println("Titel darf nicht leer sein!")
		}
	}

	var description string
	fmt.Print("Beschreibung (optional): ")
	fmt.Scanln(&description)

	saveSession(db, title, description, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))
	defer fmt.Printf("Session '%s' gespeichert! Dauer: %v\n", title, end.Sub(start).Round(time.Second))
}

// Diese Funktion erstellt die Tabelle (falls sie noch nicht existiert)
func createTable(db *sql.DB) {
	query := `
    CREATE TABLE IF NOT EXISTS work_sessions (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
		description TEXT,
        start_time TEXT NOT NULL,
        end_time TEXT
    );`

	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}

	fmt.Println("Tabelle erstellt!")
}

// Diese Funktion speichert eine Session in der Datenbank
func saveSession(db *sql.DB, title string, description string, startTime string, endTime string) {
	query := `INSERT INTO work_sessions (title, description, start_time, end_time) VALUES (?, ?, ?, ?)`

	_, err := db.Exec(query, description, title, startTime, endTime)
	if err != nil {
		panic(err)
	}

}
