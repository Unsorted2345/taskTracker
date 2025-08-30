package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite" // Statt go-sqlite3
)

func main() {

	db, err := sql.Open("sqlite", "taskTracker.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	fmt.Println("Datenbank geöffnet!")
	createTable(db)

	// Einfaches Menü
	for {
		fmt.Println("1. Timer starten")
		fmt.Println("2. Sessions anzeigen")
		fmt.Println("3. Beenden")
		fmt.Print("Wähle eine Option: ")

		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			timer(db)
		case "2":
			listSessions(db)
		case "3":
			fmt.Println("Auf Wiedersehen!")
			return
		default:
			fmt.Println("Ungültige Eingabe!")
		}
	}
}

func timer(db *sql.DB) {
	var (
		start      time.Time
		end        time.Time
		difference time.Duration
		input      string
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
		difference = end.Sub(start).Round(time.Second)
	}

	fmt.Println("timer stopped")
	var title string
	for title == "" {
		fmt.Print("Titel eingeben: ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		title = strings.TrimSpace(scanner.Text())
		if title == "" {
			fmt.Println("Titel darf nicht leer sein!")
		}
	}

	var description string
	stundenlohn := 0.0
	fmt.Println("Stundenlohn eingeben: ")
	fmt.Scanln(&stundenlohn)

	stunden := difference.Hours()
	verdienst := math.Round((stunden*stundenlohn)*100) / 100

	fmt.Print("Beschreibung (optional): ")
	fmt.Scanln(&description)

	saveSession(db, title, description, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), difference.String(), stundenlohn, verdienst)
	defer fmt.Printf("Session '%s' gespeichert! Dauer: %v. Verdienst: %.2f€\n", title, difference, verdienst)
}

// Diese Funktion erstellt die Tabelle (falls sie noch nicht existiert)
func createTable(db *sql.DB) {
	query := `
    CREATE TABLE IF NOT EXISTS work_sessions (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
		description TEXT,
        start_time TEXT NOT NULL,
        end_time TEXT,
		difference TEXT,
		stundenlohn REAL,
		verdienst REAL
    );`

	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}

	fmt.Println("Tabelle erstellt!")
}

// Diese Funktion speichert eine Session in der Datenbank
func saveSession(db *sql.DB, title string, description string, startTime string, endTime string, difference string, stundenlohn float64, verdienst float64) {
	query := `INSERT INTO work_sessions (title, description, start_time, end_time, difference, stundenlohn, verdienst) VALUES (?, ?, ?, ?, ?, ? ,?)`

	_, err := db.Exec(query, title, description, startTime, endTime, difference, stundenlohn, verdienst)
	if err != nil {
		panic(err)
	}

}

func listSessions(db *sql.DB) {
	query := `SELECT id, title, start_time, end_time, difference, stundenlohn, verdienst FROM work_sessions ORDER BY id DESC`

	rows, err := db.Query(query)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	fmt.Println("\n=== Alle Sessions ===")
	for rows.Next() {
		var id int
		var title, startTime, endTime, difference string
		var stundenlohn, verdienst float64

		err := rows.Scan(&id, &title, &startTime, &endTime, &difference, &stundenlohn, &verdienst)
		if err != nil {
			panic(err)
		}

		fmt.Printf("ID: %d | %s | %s - %s | %s | %f | %.2f\n", id, title, startTime, endTime, difference, stundenlohn, verdienst)
	}
	fmt.Println("=====================")
}
