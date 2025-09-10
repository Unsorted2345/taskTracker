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

	"github.com/google/uuid"
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
		fmt.Println("4. Session bearbeiten")
		fmt.Println("5. Session löschen")
		fmt.Println("6. Beenden")
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
			editSession(db)
		case "5":
			deleteSession(db)
		case "6":
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
		title      string
		scanner    = bufio.NewScanner(os.Stdin)
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
		difference = end.Sub(start)
	}

	fmt.Println("timer stopped")

	for title == "" {
		fmt.Print("Titel eingeben: ")
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
	scanner = bufio.NewScanner(os.Stdin)
	scanner.Scan()
	description = strings.TrimSpace(scanner.Text())

	saveSession(db, title, description, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), int64(difference.Seconds()), stundenlohn, verdienst)
	defer fmt.Printf("Session '%s' gespeichert! Dauer: %v. Verdienst: %.2f€\n", title, difference, verdienst)
}

// Diese Funktion erstellt die Tabelle (falls sie noch nicht existiert)
func createTable(db *sql.DB) {
	query := `
    CREATE TABLE IF NOT EXISTS work_sessions (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
		uuid TEXT UNIQUE NOT NULL,
        title TEXT NOT NULL,
		description TEXT,
        start_time TEXT NOT NULL,
        end_time TEXT,
		difference INTEGER,
		stundenlohn REAL,
		verdienst REAL,
		created_by TEXT NOT NULL
    );`

	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}

	fmt.Println("Starte GUI...")
	startGUI(db)

}

func getOrCreateDeviceID() string {
	// Versuche aus Datei zu lesen
	data, err := os.ReadFile("device_id.txt")
	if err == nil {
		return string(data)
	}

	// Neue Device-ID erstellen und speichern
	deviceID := uuid.New().String()
	os.WriteFile("device_id.txt", []byte(deviceID), 0644)
	return deviceID
}

// Diese Funktion speichert eine Session in der Datenbank
func saveSession(db *sql.DB, title string, description string, startTime string, endTime string, difference int64, stundenlohn float64, verdienst float64) {
	sessionUUID := uuid.New().String()
	deviceID := getOrCreateDeviceID()

	query := `INSERT INTO work_sessions (uuid, title, description, start_time, end_time, difference, stundenlohn, verdienst, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, sessionUUID, title, description, startTime, endTime, difference, stundenlohn, verdienst, deviceID)
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
		var title, startTime, endTime string
		var difference int64
		var stundenlohn, verdienst float64

		err := rows.Scan(&id, &title, &startTime, &endTime, &difference, &stundenlohn, &verdienst)
		if err != nil {
			panic(err)
		}
		dauer := time.Duration(difference) * time.Second
		fmt.Printf("ID: %d | %s | %s - %s | Dauer: %s | Lohn: %.2f€/h | Verdienst: %.2f€\n",
			id, title, startTime, endTime, dauer.String(), stundenlohn, verdienst)
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

	difference = end.Sub(start)

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

	saveSession(db, title, description, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), int64(difference.Seconds()), stundenlohn, verdienst)
	defer fmt.Printf("Session '%s' gespeichert! Dauer: %v. Verdienst: %.2f€\n", title, difference, verdienst)
}

func showSessionByID(db *sql.DB, id int) bool {
	query := `SELECT id, uuid, title, description, start_time, end_time, difference, stundenlohn, verdienst, created_by FROM work_sessions WHERE id = ?`
	row := db.QueryRow(query, id)

	var (
		sID         int
		sessionUUID string
		title       string
		description string
		startTime   string
		endTime     string
		difference  int64
		stundenlohn float64
		verdienst   float64
		createdBy   string
	)

	err := row.Scan(&sID, &sessionUUID, &title, &description, &startTime, &endTime, &difference, &stundenlohn, &verdienst, &createdBy)
	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("Keine Session mit ID %d gefunden.\n", id)
			return false
		}
		panic(err)
	}
	dauer := time.Duration(difference) * time.Second
	fmt.Printf("ID: %d | UUID %s | Titel: %s | Beschreibung: %s | %s - %s | Dauer: %s | Lohn: %.2f€/h | Verdienst: %.2f€\n",
		id, sessionUUID, title, description, startTime, endTime, dauer.String(), stundenlohn, verdienst)
	return true
}

func deleteSession(db *sql.DB) {
	query := `DELETE FROM work_sessions WHERE id = ?`
	var id int
	var input string

	fmt.Print("Gib die ID der zu löschenden Session ein: ")
	fmt.Scanln(&id)
	fmt.Println("Bist du sicher, dass du die Session mit ID", id, "löschen möchtest?")

	if !showSessionByID(db, id) {
		return // Bricht ab und kehrt zu main zurück
	}

	fmt.Print("(j/n)")
	fmt.Scanln(&input)

	if strings.ToLower(input) == "j" {
		_, err := db.Exec(query, id)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Session mit ID %d gelöscht!\n", id)
	} else {
		fmt.Println("Löschen abgebrochen.")
		return
	}
}

func editSession(db *sql.DB) {
	var query string
	var getQuery string
	var id int
	var choice string
	var scanner = bufio.NewScanner(os.Stdin)

	fmt.Print("Gib die ID der zu bearbeitenden Session ein: ")
	fmt.Scanln(&id)
	if !showSessionByID(db, id) {
		return // Bricht ab und kehrt zu main zurück
	}
	for {
		fmt.Println("Was möchtest du bearbeiten?")
		fmt.Println("1. Titel")
		fmt.Println("2. Beschreibung")
		fmt.Println("3. Startzeit")
		fmt.Println("4. Endzeit")
		fmt.Println("5. Stundenlohn")
		fmt.Println("6. Zurück")
		fmt.Print("Wähle eine Option: ")

		fmt.Scanln(&choice)

		switch choice {
		case "1":
			var title string
			for {
				fmt.Print("Neuer Titel: ")
				scanner.Scan()
				title = strings.TrimSpace(scanner.Text())
				if title == "" {
					fmt.Println("Titel darf nicht leer sein!")
					continue
				}
				query = `UPDATE work_sessions SET title = ? WHERE id = ?`
				_, err := db.Exec(query, title, id)
				if err != nil {
					panic(err)
				}
				fmt.Println("Titel aktualisiert.")
				break
			}
		case "2":
			fmt.Print("Neue Beschreibung: ")
			scanner.Scan()
			description := strings.TrimSpace(scanner.Text())
			query = `UPDATE work_sessions SET description = ? WHERE id = ?`
			_, err := db.Exec(query, description, id)
			if err != nil {
				panic(err)
			}
			fmt.Println("Beschreibung aktualisiert.")
		case "3":
			var start, end time.Time
			var difference time.Duration
			var stundenlohn float64
			getQuery = `SELECT end_time, stundenlohn FROM work_sessions WHERE id = ?`

			row := db.QueryRow(getQuery, id)

			var endStr string
			err := row.Scan(&endStr, &stundenlohn)
			if err != nil {
				panic(err)
			}

			end, err = time.Parse("2006-01-02 15:04:05", endStr)
			if err != nil {
				fmt.Println("Ungültiges Format in der Datenbank für end_time!")
				return
			}
			for {
				fmt.Print("Neue Startzeit (YYYY-MM-DD HH:MM:SS): ")
				scanner.Scan()
				startStr := strings.TrimSpace(scanner.Text())
				var err error
				start, err = time.Parse("2006-01-02 15:04:05", startStr)
				if err != nil {
					fmt.Println("Ungültiges Format! Bitte erneut eingeben.")
					continue
				}
				difference = end.Sub(start).Round(time.Second)
				stunden := difference.Hours()
				verdienst := math.Round((stunden*stundenlohn)*100) / 100
				query = `UPDATE work_sessions SET start_time = ?, difference = ?, verdienst = ? WHERE id = ?`
				_, err = db.Exec(query, start.Format("2006-01-02 15:04:05"), int64(difference.Seconds()), verdienst, id)
				if err != nil {
					panic(err)
				}
				fmt.Println("Startzeit aktualisiert.")
				break
			}
		case "4":
			var start, end time.Time
			var difference time.Duration
			var stundenlohn float64
			getQuery = `SELECT start_time, stundenlohn FROM work_sessions WHERE id = ?`

			row := db.QueryRow(getQuery, id)

			var startStr string
			err := row.Scan(&startStr, &stundenlohn)
			if err != nil {
				panic(err)
			}

			start, err = time.Parse("2006-01-02 15:04:05", startStr)
			if err != nil {
				fmt.Println("Ungültiges Format in der Datenbank für start_time!")
				return
			}
			for {
				fmt.Print("Neue Endzeit (YYYY-MM-DD HH:MM:SS): ")
				scanner.Scan()
				endStr := strings.TrimSpace(scanner.Text())
				var err error
				end, err = time.Parse("2006-01-02 15:04:05", endStr)
				if err != nil {
					fmt.Println("Ungültiges Format! Bitte erneut eingeben.")
					continue
				}
				difference = end.Sub(start).Round(time.Second)
				stunden := difference.Hours()
				verdienst := math.Round((stunden*stundenlohn)*100) / 100
				query = `UPDATE work_sessions SET end_time = ?, difference = ?, verdienst = ? WHERE id = ?`
				_, err = db.Exec(query, end.Format("2006-01-02 15:04:05"), int64(difference.Seconds()), verdienst, id)
				if err != nil {
					panic(err)
				}
				fmt.Println("Endzeit aktualisiert.")
				break
			}
		case "5":
			var differenceSeconds int64
			getQuery = `SELECT difference FROM work_sessions WHERE id = ?`
			row := db.QueryRow(getQuery, id)
			err := row.Scan(&differenceSeconds)
			if err != nil {
				panic(err)
			}
			difference := time.Duration(differenceSeconds) * time.Second

			for {
				fmt.Print("Neuer Stundenlohn: ")
				scanner.Scan()
				stundenlohnStr := strings.TrimSpace(scanner.Text())
				stundenlohn, err := strconv.ParseFloat(stundenlohnStr, 64)
				if err != nil {
					fmt.Println("Ungültiger Stundenlohn! Bitte erneut eingeben.")
					continue
				}
				stunden := difference.Hours()
				verdienst := math.Round((stunden*stundenlohn)*100) / 100
				query = `UPDATE work_sessions SET stundenlohn = ?, verdienst = ? WHERE id = ?`
				_, err = db.Exec(query, stundenlohn, verdienst, id)
				if err != nil {
					panic(err)
				}
				fmt.Println("Stundenlohn aktualisiert.")
				break
			}
		case "6":
			return
		default:
			fmt.Println("Ungültige Eingabe!")
		}
	}
}
