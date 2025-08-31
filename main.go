package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strconv"
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
		fmt.Println("2. Session manuell hinzufügen")
		fmt.Println("3. Sessions anzeigen")
		fmt.Println("4. Beenden")
		fmt.Print("Wähle eine Option: ")

		var choice string
		fmt.Scanln(&choice)

		switch choice {
		case "1":
			timer(db)
		case "2":
			addSession(db)
		case "3":
			listSessions(db)
		case "4":
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
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	description = strings.TrimSpace(scanner.Text())

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

func addSession(db *sql.DB) {
	var (
		start, end  time.Time
		difference  time.Duration
		title       string
		description string
		stundenlohn float64
	)

	scanner := bufio.NewScanner(os.Stdin)

	for title == "" {
		fmt.Print("Titel eingeben: ")
		scanner.Scan()
		title = strings.TrimSpace(scanner.Text())
		if title == "" {
			fmt.Println("Titel darf nicht leer sein!")
		}
	}

	fmt.Print("Beschreibung (optional): ")
	scanner.Scan()
	description = strings.TrimSpace(scanner.Text())

	fmt.Print("Startzeit (YYYY-MM-DD HH:MM:SS): ")
	scanner.Scan()
	startStr := strings.TrimSpace(scanner.Text())
	var err error
	start, err = time.Parse("2006-01-02 15:04:05", startStr)
	if err != nil {
		fmt.Println("Ungültiges Format für Startzeit!")
		return
	}

	fmt.Print("Endzeit (YYYY-MM-DD HH:MM:SS): ")
	scanner.Scan()
	endStr := strings.TrimSpace(scanner.Text())
	end, err = time.Parse("2006-01-02 15:04:05", endStr)
	if err != nil {
		fmt.Println("Ungültiges Format für Endzeit!")
		return
	}

	difference = end.Sub(start).Round(time.Second)

	fmt.Print("Stundenlohn eingeben: ")
	scanner.Scan()
	stundenlohnStr := strings.TrimSpace(scanner.Text())
	stundenlohn, err = strconv.ParseFloat(stundenlohnStr, 64)
	if err != nil {
		fmt.Println("Ungültiger Stundenlohn!")
		return
	}

	stunden := difference.Hours()
	verdienst := math.Round((stunden*stundenlohn)*100) / 100

	saveSession(db, title, description, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), difference.String(), stundenlohn, verdienst)
	defer fmt.Printf("Session '%s' gespeichert! Dauer: %v. Verdienst: %.2f€\n", title, difference, verdienst)
}
