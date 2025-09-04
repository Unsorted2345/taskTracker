package main

import (
	"database/sql"
	"math"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func startGUI(db *sql.DB) {
	myApp := app.New()
	myWindow := myApp.NewWindow("TaskTracker")
	myWindow.Resize(fyne.NewSize(800, 600))

	// Haupt-Tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("Timer", createTimerTab(db)),
		container.NewTabItem("Sessions", createSessionsTab(db)),
		container.NewTabItem("Hinzufügen", createAddTab(db)),
		container.NewTabItem("Bearbeiten", createEditTab(db)),
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
		statusLabel.SetText("Session gespeichert")
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
		container.NewHBox(startButton, stopButton),
		widget.NewSeparator(),
		title,
		description,
		stundenlohn,
		saveButton,
	)
}

func createSessionsTab(db *sql.DB) fyne.CanvasObject {
	// Sessions anzeigen
	return container.NewVBox(
		widget.NewLabel("Alle Sessions"),
	)
}

func createAddTab(db *sql.DB) fyne.CanvasObject {
	// Session hinzufügen
	return container.NewVBox(
		widget.NewLabel("Session hinzufügen"),
	)
}

func createEditTab(db *sql.DB) fyne.CanvasObject {
	// Session bearbeiten
	return container.NewVBox(
		widget.NewLabel("Session bearbeiten"),
	)
}
