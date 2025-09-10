package main

import (
	"database/sql"
	"fmt"
	"image/color"
	"math"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type Session struct {
	id          int
	title       string
	description string
	startTime   string
	endTime     string
	difference  int64
	stundenlohn float64
	verdienst   float64
	createdBy   string
}

func startGUI(db *sql.DB) {
	myApp := app.New()
	myWindow := myApp.NewWindow("TaskTracker")
	myWindow.Resize(fyne.NewSize(800, 600))

	// Haupt-Tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("Timer", createTimerTab(db)),
		container.NewTabItem("Sessions", createSessionsTab(db)),
		container.NewTabItem("Hinzufügen", createAddSessionTab(db)),
		container.NewTabItem("Bearbeiten", createEditSessionTab(db)),
		container.NewTabItem("Löschen", createDeleteSessionTab(db)),
	)

	myWindow.SetContent(tabs)
	myWindow.ShowAndRun()
}

func createTimerTab(db *sql.DB) fyne.CanvasObject {
	var start, end time.Time
	var difference time.Duration
	var startButton, stopButton, saveButton *widget.Button
	var title, description *widget.Entry
	var stundenlohn *widget.Entry
	statusLabel := widget.NewLabel("Timer bereit")

	title = widget.NewEntry()
	description = widget.NewEntry()
	stundenlohn = widget.NewEntry()

	// Timer Buttons zuerst initialisieren!
	startButton = widget.NewButton("Timer starten", func() {
		statusLabel.SetText("Timer läuft...")
		startButton.Hide()
		stopButton.Show()
		start = time.Now()
	})
	stopButton = widget.NewButton("Timer stoppen", func() {
		statusLabel.SetText("Timer gestoppt")
		stopButton.Hide()
		title.Show()
		description.Show()
		stundenlohn.Show()
		saveButton.Show()
		end = time.Now()
		difference = end.Sub(start)
	})

	saveButton = widget.NewButton("Session speichern", func() {
		title.Hide()
		description.Hide()
		stundenlohn.Hide()
		saveButton.Hide()
		startButton.Show()
		stundenlohnWert, err := strconv.ParseFloat(stundenlohn.Text, 64)
		if err != nil {
			statusLabel.SetText("Ungültiger Stundenlohn!")
			return
		}
		stunden := difference.Hours()
		verdienst := math.Round((stunden*stundenlohnWert)*100) / 100
		saveSession(db, title.Text, description.Text, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), int64(difference.Seconds()), stundenlohnWert, verdienst)
		defer statusLabel.SetText("Session '" + title.Text + "' gespeichert! Dauer: " + difference.String() + ". Verdienst: " + strconv.FormatFloat(verdienst, 'f', 2, 64) + "€")
		title.SetText("")
		description.SetText("")
		stundenlohn.SetText("")
	})

	// Jetzt erst verstecken!
	title.Hide()
	description.Hide()
	stundenlohn.Hide()
	saveButton.Hide()
	stopButton.Hide()

	title.SetPlaceHolder("Titel eingeben...")
	description.SetPlaceHolder("Beschreibung (optional)")
	description.MultiLine = true
	stundenlohn.SetPlaceHolder("Stundenlohn (€)")

	return container.NewVBox(
		statusLabel,
		startButton,
		stopButton,
		widget.NewSeparator(),
		title,
		description,
		stundenlohn,
		saveButton,
	)
}

func createSessionsTab(db *sql.DB) fyne.CanvasObject {
	sessionsList := container.NewVBox()
	line := canvas.NewLine(color.NRGBA{R: 41, G: 111, B: 246, A: 255})
	line.StrokeWidth = 3
	countLabel := widget.NewLabel("Anzahl: 0")
	totalLabel := widget.NewLabel("Gesamtverdienst: 0€")
	var refreshBtn *widget.Button

	loadSessions := func() {
		sessionsList.RemoveAll()
		sessions := returnSessions(db)

		for _, s := range sessions {
			// Card-Container für jede Session
			timeLabel := widget.NewLabel("Zeit: " + s.startTime + " - " + s.endTime)
			durationLabel := widget.NewLabel("Dauer: " + (time.Duration(s.difference) * time.Second).String())
			earningsLabel := widget.NewLabel("Verdienst: " + strconv.FormatFloat(s.verdienst, 'f', 2, 64) + "€")

			// Card als VBox
			sessionCard := widget.NewCard("", s.title, container.NewVBox(
				timeLabel,
				durationLabel,
				earningsLabel,
				widget.NewLabel("Beschreibung: "+s.description),
				widget.NewLabel("Erstellt von: "+s.createdBy),
				line,
			))

			sessionsList.Add(sessionCard)
		}
		// Statistiken aktualisieren...
	}

	refreshBtn = widget.NewButton("Aktualisieren", func() {
		loadSessions()
	})

	loadSessions()

	// Scroll um die komplette sessionsList
	scrollableContent := container.NewScroll(sessionsList)

	return container.NewBorder(
		container.NewVBox(refreshBtn, widget.NewLabel("Alle Sessions")), // top
		container.NewVBox(countLabel, totalLabel),                       // bottom
		nil,               // left
		nil,               // right
		scrollableContent, // center (nimmt verfügbaren Platz)
	)
}

func createAddSessionTab(db *sql.DB) fyne.CanvasObject {
	var addSessionButton, saveButton *widget.Button
	var title, description, startEntry, endEntry, stundenlohn *widget.Entry
	statusLabel := widget.NewLabel("Session hinzufügen")
	var difference time.Duration
	var start, end time.Time

	title = widget.NewEntry()
	description = widget.NewEntry()
	startEntry = widget.NewEntry()
	endEntry = widget.NewEntry()
	stundenlohn = widget.NewEntry()

	addSessionButton = widget.NewButton("Neue Session hinzufügen", func() {
		title.Show()
		description.Show()
		startEntry.Show()
		endEntry.Show()
		stundenlohn.Show()
		saveButton.Show()
		addSessionButton.Hide()
		statusLabel.SetText("Bitte Session-Daten eingeben")
	})

	saveButton = widget.NewButton("Änderungen speichern", func() {
		title.Hide()
		description.Hide()
		startEntry.Hide()
		endEntry.Hide()
		stundenlohn.Hide()
		saveButton.Hide()
		addSessionButton.Show()

		// Zeit parsen
		var err error
		start, err = time.Parse("2006-01-02 15:04:05", startEntry.Text)
		if err != nil {
			statusLabel.SetText("Ungültiges Startzeit-Format!")
			return
		}
		end, err = time.Parse("2006-01-02 15:04:05", endEntry.Text)
		if err != nil {
			statusLabel.SetText("Ungültiges Endzeit-Format!")
			return
		}
		difference = end.Sub(start)

		stundenlohnWert, err := strconv.ParseFloat(stundenlohn.Text, 64)
		if err != nil {
			statusLabel.SetText("Ungültiger Stundenlohn!")
			return
		}
		stunden := difference.Hours()
		verdienst := math.Round((stunden*stundenlohnWert)*100) / 100

		saveSession(db, title.Text, description.Text, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), int64(difference.Seconds()), stundenlohnWert, verdienst)
		statusLabel.SetText("Session '" + title.Text + "' gespeichert! Dauer: " + difference.String() + ". Verdienst: " + strconv.FormatFloat(verdienst, 'f', 2, 64) + "€")
		title.SetText("")
		description.SetText("")
		startEntry.SetText("")
		endEntry.SetText("")
		stundenlohn.SetText("")

	})

	saveButton.Hide()
	title.Hide()
	description.Hide()
	startEntry.Hide()
	endEntry.Hide()
	stundenlohn.Hide()

	title.SetPlaceHolder("Titel eingeben...")
	description.SetPlaceHolder("Beschreibung (optional)")
	description.MultiLine = true
	startEntry.SetPlaceHolder("Start (YYYY-MM-DD HH:MM:SS)")
	endEntry.SetPlaceHolder("Ende (YYYY-MM-DD HH:MM:SS)")
	stundenlohn.SetPlaceHolder("Stundenlohn (€)")

	return container.NewVBox(
		statusLabel,
		addSessionButton,
		title,
		description,
		startEntry,
		endEntry,
		stundenlohn,
		saveButton,
	)
}

func createEditSessionTab(db *sql.DB) fyne.CanvasObject {
	var id *widget.Entry
	var editBtn, editTitleBtn, editDescBtn, editStartBtn, editEndBtn, editStundenlohnnBtn, confirmTitleBtn, confirmDescBtn, confirmStartBtn, confirmEndBtn, confirmStundenlohnBtn *widget.Button
	var newTitle, newDesc, newStart, newEnd, newStundenlohn *widget.Entry
	var outputLabel *widget.Label
	var query, getQuery string

	id = widget.NewEntry()
	id.SetPlaceHolder("Session ID eingeben...")

	newTitle = widget.NewEntry()
	newDesc = widget.NewEntry()
	newStart = widget.NewEntry()
	newEnd = widget.NewEntry()
	newStundenlohn = widget.NewEntry()

	// outputLabel initialisieren, sonst panic beim Layouten (nil pointer)
	outputLabel = widget.NewLabel("")

	editBtn = widget.NewButton("Session laden", func() {
		sessionID, err := strconv.Atoi(id.Text)
		if err != nil {
			outputLabel.SetText("Ungültige ID!")
			return
		}
		session, found := returnSessionByID(db, sessionID)
		if !found {
			outputLabel.SetText(session) // Fehlermeldung anzeigen
			return
		}
		id.Hide()
		editBtn.Hide()
		editTitleBtn.Show()
		editDescBtn.Show()
		editStartBtn.Show()
		editEndBtn.Show()
		editStundenlohnnBtn.Show()
		outputLabel.SetText("Was möchtest du an dieser Session änder? " + session) // Session-Daten anzeigen
	})

	editTitleBtn = widget.NewButton("Titel bearbeiten", func() {
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editStartBtn.Hide()
		editEndBtn.Hide()
		editStundenlohnnBtn.Hide()
		newTitle.Show()
		newTitle.SetPlaceHolder("Neuer Titel...")
		confirmTitleBtn.Show()
	})

	editDescBtn = widget.NewButton("Beschreibung bearbeiten", func() {
		editDescBtn.Hide()
		editTitleBtn.Hide()
		editStartBtn.Hide()
		editEndBtn.Hide()
		editStundenlohnnBtn.Hide()
		newDesc.Show()
		newDesc.SetPlaceHolder("Neue Beschreibung...")
		confirmDescBtn.Show()
	})

	editStartBtn = widget.NewButton("Startzeit bearbeiten", func() {
		editStartBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editEndBtn.Hide()
		editStundenlohnnBtn.Hide()
		newStart.Show()
		newStart.SetPlaceHolder("Neuer Start (YYYY-MM-DD HH:MM:SS)")
		confirmStartBtn.Show()
	})

	editEndBtn = widget.NewButton("Endzeit bearbeiten", func() {
		editEndBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editStartBtn.Hide()
		editStundenlohnnBtn.Hide()
		newEnd.Show()
		newEnd.SetPlaceHolder("Neues Ende (YYYY-MM-DD HH:MM:SS)")
		confirmEndBtn.Show()
	})

	editStundenlohnnBtn = widget.NewButton("Stundenlohn bearbeiten", func() {
		editStundenlohnnBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editStartBtn.Hide()
		editEndBtn.Hide()
		newStundenlohn.Show()
		newStundenlohn.SetPlaceHolder("Neuer Stundenlohn (€)")
		confirmStundenlohnBtn.Show()
	})

	confirmTitleBtn = widget.NewButton("Titel speichern", func() {
		query = "UPDATE work_sessions SET title = ? WHERE id = ?"

		_, err := db.Exec(query, newTitle.Text, id.Text)
		if err != nil {
			outputLabel.SetText("Fehler beim Speichern des Titels: " + err.Error())
			return
		}
		outputLabel.SetText("Titel erfolgreich aktualisiert!")
		newTitle.SetText("")
		newTitle.Hide()
		confirmTitleBtn.Hide()
		id.Show()
		editBtn.Show()
	})

	confirmDescBtn = widget.NewButton("Beschreibung speichern", func() {
		query = "UPDATE work_sessions SET description = ? WHERE id = ?"
		_, err := db.Exec(query, newDesc.Text, id.Text)
		if err != nil {
			outputLabel.SetText("Fehler beim Speichern der Beschreibung: " + err.Error())
			return
		}
		outputLabel.SetText("Beschreibung erfolgreich aktualisiert!")
		newDesc.SetText("")
		newDesc.Hide()
		confirmDescBtn.Hide()
		id.Show()
		editBtn.Show()
	})

	confirmStartBtn = widget.NewButton("Startzeit speichern", func() {
		query = "UPDATE work_sessions SET start_time = ?, difference = ?, verdienst = ? WHERE id = ?"
		getQuery = "SELECT end_time, stundenlohn FROM work_sessions WHERE id = ?"
		row := db.QueryRow(getQuery, id.Text)
		var endStr string
		var stundenlohn float64
		var end, start time.Time

		// erstes Scan holen (einmalig)
		err := row.Scan(&endStr, &stundenlohn)
		if err != nil {
			panic(err)
		}

		end, err = time.Parse("2006-01-02 15:04:05", endStr)
		if err != nil {
			outputLabel.SetText("Ungültiges Format in der Datenbank für end_time!")
			return
		}

		start, err = time.Parse("2006-01-02 15:04:05", newStart.Text)
		if err != nil {
			outputLabel.SetText("Ungültiges Startzeit-Format!")
			return
		}

		// Update start_time, difference und verdienst und Fehler prüfen
		difference := end.Sub(start)
		verdienst := math.Round((difference.Hours()*stundenlohn)*100) / 100

		_, err = db.Exec(query, newStart.Text, int64(difference.Seconds()), verdienst, id.Text)
		if err != nil {
			outputLabel.SetText("Fehler beim Aktualisieren: " + err.Error())
			return
		}

		outputLabel.SetText("Startzeit erfolgreich aktualisiert!")
		newStart.SetText("")
		newStart.Hide()
		confirmStartBtn.Hide()
		id.Show()
		editBtn.Show()
	})

	confirmEndBtn = widget.NewButton("Endzeit speichern", func() {
		query = "UPDATE work_sessions SET end_time = ?, difference = ?, verdienst = ? WHERE id = ?"
		getQuery = "SELECT start_time, stundenlohn FROM work_sessions WHERE id = ?"
		row := db.QueryRow(getQuery, id.Text)
		var startStr string
		var stundenlohn float64
		var end, start time.Time

		// erstes Scan holen (einmalig)
		err := row.Scan(&startStr, &stundenlohn)
		if err != nil {
			panic(err)
		}

		start, err = time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			outputLabel.SetText("Ungültiges Format in der Datenbank für start_time!")
			return
		}

		end, err = time.Parse("2006-01-02 15:04:05", newEnd.Text)
		if err != nil {
			outputLabel.SetText("Ungültiges Endzeit-Format!")
			return
		}

		// Update start_time, difference und verdienst und Fehler prüfen
		difference := end.Sub(start)
		verdienst := math.Round((difference.Hours()*stundenlohn)*100) / 100

		_, err = db.Exec(query, newEnd.Text, int64(difference.Seconds()), verdienst, id.Text)
		if err != nil {
			outputLabel.SetText("Fehler beim Aktualisieren: " + err.Error())
			return
		}

		outputLabel.SetText("Endzeit erfolgreich aktualisiert!")
		newEnd.SetText("")
		newEnd.Hide()
		confirmEndBtn.Hide()
		id.Show()
		editBtn.Show()
	})

	confirmStundenlohnBtn = widget.NewButton("Stundenlohn speichern", func() {
		getQuery = "SELECT difference FROM work_sessions WHERE id = ?"
		query = "UPDATE work_sessions SET stundenlohn = ?, verdienst = ? WHERE id = ?"
		var differenceSeconds int64

		row := db.QueryRow(getQuery, id.Text)
		err := row.Scan(&differenceSeconds)
		if err != nil {
			panic(err)
		}
		difference := time.Duration(differenceSeconds) * time.Second
		stunden := difference.Hours()
		newStundenlohnWert, err := strconv.ParseFloat(newStundenlohn.Text, 64)
		if err != nil {
			outputLabel.SetText("Ungültiger Stundenlohn!")
			return
		}
		verdienst := math.Round((stunden*newStundenlohnWert)*100) / 100

		_, err = db.Exec(query, newStundenlohn.Text, verdienst, id.Text)
		if err != nil {
			outputLabel.SetText("Fehler beim Speichern des Stundenlohns: " + err.Error())
			return
		}
		outputLabel.SetText("Stundenlohn erfolgreich aktualisiert!")
		newStundenlohn.SetText("")
		newStundenlohn.Hide()
		confirmStundenlohnBtn.Hide()
		id.Show()
		editBtn.Show()
	})

	editTitleBtn.Hide()
	editDescBtn.Hide()
	editStartBtn.Hide()
	editEndBtn.Hide()
	editStundenlohnnBtn.Hide()
	newTitle.Hide()
	newDesc.Hide()
	newStart.Hide()
	newEnd.Hide()
	newStundenlohn.Hide()
	confirmTitleBtn.Hide()
	confirmDescBtn.Hide()
	confirmStartBtn.Hide()
	confirmEndBtn.Hide()
	confirmStundenlohnBtn.Hide()

	return container.NewVBox(
		widget.NewLabel("Session bearbeiten"),
		id,
		editBtn,
		outputLabel,
		editTitleBtn,
		editDescBtn,
		editStartBtn,
		editEndBtn,
		editStundenlohnnBtn,
		newTitle,
		newDesc,
		newStart,
		newEnd,
		newStundenlohn,
		confirmTitleBtn,
		confirmDescBtn,
		confirmStartBtn,
		confirmEndBtn,
		confirmStundenlohnBtn,
	)
}

func createDeleteSessionTab(db *sql.DB) fyne.CanvasObject {
	var id *widget.Entry
	var deleteBtn, confirmDeleteBtn *widget.Button
	var outputLabel *widget.Label
	query := "DELETE FROM work_sessions WHERE id = ?"

	id = widget.NewEntry()
	id.SetPlaceHolder("Session ID eingeben...")
	outputLabel = widget.NewLabel("")

	deleteBtn = widget.NewButton("Session laden", func() {
		sessionID, err := strconv.Atoi(id.Text)
		if err != nil {
			outputLabel.SetText("Ungültige ID!")
			return
		}
		session, found := returnSessionByID(db, sessionID)
		if !found {
			outputLabel.SetText(session) // Fehlermeldung anzeigen
			return
		}
		outputLabel.SetText("Möchtest du diese Session wirklich löschen? " + session) // Session-Daten anzeigen
		deleteBtn.Hide()
		confirmDeleteBtn.Show()
	})

	confirmDeleteBtn = widget.NewButton("Session löschen", func() {
		_, err := db.Exec(query, id.Text)
		if err != nil {
			outputLabel.SetText("Fehler beim Löschen der Session: " + err.Error())
			return
		}
		outputLabel.SetText("Session erfolgreich gelöscht!")
		id.SetText("")
		id.Show()
		deleteBtn.Show()
		confirmDeleteBtn.Hide()
	})

	confirmDeleteBtn.Hide()

	return container.NewVBox(
		widget.NewLabel("Session löschen"),
		id,
		deleteBtn,
		confirmDeleteBtn,
		outputLabel,
	)
}

func returnSessions(db *sql.DB) []Session {
	query := "SELECT id, title, description, start_time, end_time, difference, stundenlohn, verdienst, created_by FROM work_sessions ORDER BY end_time DESC"
	rows, err := db.Query(query)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		err := rows.Scan(&s.id, &s.title, &s.description, &s.startTime, &s.endTime, &s.difference, &s.stundenlohn, &s.verdienst, &s.createdBy)
		if err != nil {
			panic(err)
		}
		sessions = append(sessions, s)
	}
	return sessions
}

func returnSessionByID(db *sql.DB, id int) (Session string, status bool) {
	query := "SELECT id, uuid, title, description, start_time, end_time, difference, stundenlohn, verdienst, created_by FROM work_sessions WHERE id = ?"
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
			session := fmt.Sprintf("Keine Session mit ID %d gefunden.\n", id)
			return session, false
		}
		panic(err)
	}

	session := fmt.Sprintf("ID: %d | UUID %s | Titel: %s | Beschreibung: %s | %s - %s | Dauer: %s | Lohn: %.2f€/h | Verdienst: %.2f€ | Erstellt von: %s",
		sID, sessionUUID, title, description, startTime, endTime, (time.Duration(difference) * time.Second).String(), stundenlohn, verdienst, createdBy)
	return session, true
}
