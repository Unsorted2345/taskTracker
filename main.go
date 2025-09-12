package main

import (
	"database/sql"
	"fmt"
	"image/color"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Session represents one work session stored in the database.
// Difference is stored as seconds (int64) and Earnings as float64.
type Session struct {
	ID          int
	Title       string
	Description string
	StartTime   string
	EndTime     string
	Difference  int64
	HourlyRate  float64
	Earnings    float64
	CreatedBy   string
}

func main() {
	// create application and main window
	myApp := app.New()
	myWindow := myApp.NewWindow("TaskTracker")
	myWindow.Resize(fyne.NewSize(800, 600))

	// open sqlite database (modernc.org/sqlite driver)
	db, err := sql.Open("sqlite", "taskTracker.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// ensure table exists
	createTable(db)

	// create application tabs and set content
	tabs := container.NewAppTabs(
		container.NewTabItem("Timer", createTimerTab(db)),
		container.NewTabItem("Sessions", createSessionsTab(db)),
		container.NewTabItem("Add", createAddSessionTab(db)),
		container.NewTabItem("Edit", createEditSessionTab(db)),
		container.NewTabItem("Delete", createDeleteSessionTab(db)),
	)

	myWindow.SetContent(tabs)
	myWindow.ShowAndRun()
}

// createTimerTab builds the timer UI where user can start/stop and save a session.
// The timer calculates duration (time.Duration) and earnings before saving.
func createTimerTab(db *sql.DB) fyne.CanvasObject {
	var start, end time.Time   // runtime start/end time
	var duration time.Duration // computed duration
	var startBtn, stopBtn, saveBtn *widget.Button
	var titleEntry, descEntry, hourlyRateEntry *widget.Entry

	// status label shows messages to the user
	statusLabel := widget.NewLabel("Timer ready")

	// create and configure entry widgets
	titleEntry = widget.NewEntry()
	descEntry = widget.NewEntry()
	hourlyRateEntry = widget.NewEntry()

	// create start button
	startBtn = widget.NewButton("Start timer", func() {
		statusLabel.SetText("Timer running...")
		startBtn.Hide()
		stopBtn.Show()
		start = time.Now()
	})

	// create stop button
	stopBtn = widget.NewButton("Stop timer", func() {
		statusLabel.SetText("Timer stopped")
		stopBtn.Hide()
		titleEntry.Show()
		descEntry.Show()
		hourlyRateEntry.Show()
		saveBtn.Show()
		end = time.Now()
		duration = end.Sub(start)
	})

	// create save button (converts inputs and persists session)
	saveBtn = widget.NewButton("Save session", func() {
		// hide input widgets and show start again
		titleEntry.Hide()
		descEntry.Hide()
		hourlyRateEntry.Hide()
		saveBtn.Hide()
		startBtn.Show()

		// parse hourly rate from entry
		hourlyRate, err := strconv.ParseFloat(hourlyRateEntry.Text, 64)
		if err != nil {
			statusLabel.SetText("Invalid hourly rate!")
			return
		}

		// compute earnings and round to 2 decimals
		hours := duration.Hours()
		earnings := math.Round((hours*hourlyRate)*100) / 100

		// save session (start/end passed as time.Time)
		err = saveSession(db, titleEntry.Text, descEntry.Text, start, end, int64(duration.Seconds()), hourlyRate, earnings)
		if err != nil {
			statusLabel.SetText("Error saving session: " + err.Error())
			return
		}

		// inform user and reset inputs
		statusLabel.SetText(fmt.Sprintf("Session '%s' saved. Duration: %s. Earnings: %.2f€", titleEntry.Text, duration.String(), earnings))
		titleEntry.SetText("")
		descEntry.SetText("")
		hourlyRateEntry.SetText("")
	})

	// hide input elements initially
	titleEntry.Hide()
	descEntry.Hide()
	hourlyRateEntry.Hide()
	saveBtn.Hide()
	stopBtn.Hide()

	// placeholders and configuration
	titleEntry.SetPlaceHolder("Enter title...")
	descEntry.SetPlaceHolder("Description (optional)")
	descEntry.MultiLine = true
	hourlyRateEntry.SetPlaceHolder("Hourly rate (€)")

	// layout: status, buttons row, separator, inputs
	return container.NewVBox(
		statusLabel,
		container.NewHBox(startBtn, stopBtn),
		widget.NewSeparator(),
		titleEntry,
		descEntry,
		hourlyRateEntry,
		saveBtn,
	)
}

// createSessionsTab builds the view that lists saved sessions.
// It shows count and total earnings and supports manual refresh.
func createSessionsTab(db *sql.DB) fyne.CanvasObject {
	// vertical container to hold session cards
	sessionsList := container.NewVBox()

	// decorative divider line (canvas element)
	divider := canvas.NewLine(color.NRGBA{R: 41, G: 111, B: 246, A: 255})
	divider.StrokeWidth = 3

	// summary labels
	countLabel := widget.NewLabel("Count: 0")
	totalLabel := widget.NewLabel("Total earnings: 0.00€")
	var refreshBtn *widget.Button

	// loader function to refill sessionsList from DB
	loadSessions := func() {
		sessionsList.RemoveAll()
		sessions := getAllSessions(db)

		var total float64
		for _, s := range sessions {
			total += s.Earnings

			// create small labels for each session row
			timeLabel := widget.NewLabel("Time: " + s.StartTime + " - " + s.EndTime)
			durationLabel := widget.NewLabel("Duration: " + (time.Duration(s.Difference) * time.Second).String())
			earningsLabel := widget.NewLabel("Earnings: " + strconv.FormatFloat(s.Earnings, 'f', 2, 64) + "€")

			// pack into a card for better visual separation
			card := widget.NewCard(s.Title, "", container.NewVBox(
				timeLabel,
				durationLabel,
				earningsLabel,
				widget.NewLabel("Description: "+s.Description),
				widget.NewLabel("Created by: "+s.CreatedBy),
				divider,
			))
			sessionsList.Add(card)
		}
		// update summary labels
		countLabel.SetText("Count: " + strconv.Itoa(len(sessions)))
		totalLabel.SetText("Total earnings: " + strconv.FormatFloat(total, 'f', 2, 64) + "€")
	}

	// refresh button to reload data
	refreshBtn = widget.NewButton("Refresh", func() {
		loadSessions()
	})

	// initial load
	loadSessions()

	// make sessions list scrollable
	scrollable := container.NewScroll(sessionsList)

	// layout: top-left controls, bottom summary, center scroll area
	return container.NewBorder(
		container.NewVBox(refreshBtn, widget.NewLabel("All sessions")),
		container.NewVBox(countLabel, totalLabel),
		nil,
		nil,
		scrollable,
	)
}

// createAddSessionTab provides UI to add a session by manually entering start and end times.
func createAddSessionTab(db *sql.DB) fyne.CanvasObject {
	var addBtn, saveBtn *widget.Button
	var titleEntry, descEntry, startEntry, endEntry, hourlyRateEntry *widget.Entry
	statusLabel := widget.NewLabel("Add session")
	var start, end time.Time
	var duration time.Duration

	// create entries
	titleEntry = widget.NewEntry()
	descEntry = widget.NewEntry()
	startEntry = widget.NewEntry()
	endEntry = widget.NewEntry()
	hourlyRateEntry = widget.NewEntry()

	// new session button: reveal inputs
	addBtn = widget.NewButton("New session", func() {
		titleEntry.Show()
		descEntry.Show()
		startEntry.Show()
		endEntry.Show()
		hourlyRateEntry.Show()
		saveBtn.Show()
		addBtn.Hide()
		statusLabel.SetText("Please enter session data")
	})

	// save handler: parse times and rate, compute earnings and persist
	saveBtn = widget.NewButton("Save", func() {
		// hide inputs and show add button again
		titleEntry.Hide()
		descEntry.Hide()
		startEntry.Hide()
		endEntry.Hide()
		hourlyRateEntry.Hide()
		saveBtn.Hide()
		addBtn.Show()

		var err error
		start, err = time.Parse("2006-01-02 15:04:05", startEntry.Text)
		if err != nil {
			statusLabel.SetText("Invalid start time format!")
			return
		}
		end, err = time.Parse("2006-01-02 15:04:05", endEntry.Text)
		if err != nil {
			statusLabel.SetText("Invalid end time format!")
			return
		}
		duration = end.Sub(start)

		hourlyRate, err := strconv.ParseFloat(hourlyRateEntry.Text, 64)
		if err != nil {
			statusLabel.SetText("Invalid hourly rate!")
			return
		}
		hours := duration.Hours()
		earnings := math.Round((hours*hourlyRate)*100) / 100

		// save to DB (saveSession returns an error we surface to user)
		err = saveSession(db, titleEntry.Text, descEntry.Text, start, end, int64(duration.Seconds()), hourlyRate, earnings)
		if err != nil {
			statusLabel.SetText("Error saving session: " + err.Error())
			return
		}

		// success message and clear fields
		statusLabel.SetText(fmt.Sprintf("Saved '%s'. Duration: %s. Earnings: %.2f€", titleEntry.Text, duration.String(), earnings))
		titleEntry.SetText("")
		descEntry.SetText("")
		startEntry.SetText("")
		endEntry.SetText("")
		hourlyRateEntry.SetText("")
	})

	// hide inputs initially
	saveBtn.Hide()
	titleEntry.Hide()
	descEntry.Hide()
	startEntry.Hide()
	endEntry.Hide()
	hourlyRateEntry.Hide()

	// placeholders
	titleEntry.SetPlaceHolder("Title...")
	descEntry.SetPlaceHolder("Description (optional)")
	descEntry.MultiLine = true
	startEntry.SetPlaceHolder("Start (YYYY-MM-DD HH:MM:SS)")
	endEntry.SetPlaceHolder("End (YYYY-MM-DD HH:MM:SS)")
	hourlyRateEntry.SetPlaceHolder("Hourly rate (€)")

	return container.NewVBox(
		statusLabel,
		addBtn,
		titleEntry,
		descEntry,
		startEntry,
		endEntry,
		hourlyRateEntry,
		saveBtn,
	)
}

// createEditSessionTab lets the user load a session by ID and edit individual fields.
// It reads the current values from the database and updates only selected columns.
func createEditSessionTab(db *sql.DB) fyne.CanvasObject {
	var idEntry *widget.Entry
	var loadBtn *widget.Button
	var editTitleBtn, editDescBtn, editStartBtn, editEndBtn, editHourlyRateBtn *widget.Button
	var newTitle, newDesc, newStart, newEnd, newHourlyRate *widget.Entry

	// output label displays messages or loaded session summary
	outputLabel := widget.NewLabel("")
	var getQuery, updateQuery string

	// input for session id
	idEntry = widget.NewEntry()
	idEntry.SetPlaceHolder("Enter session ID...")

	// entries for new values
	newTitle = widget.NewEntry()
	newDesc = widget.NewEntry()
	newStart = widget.NewEntry()
	newEnd = widget.NewEntry()
	newHourlyRate = widget.NewEntry()

	// load button: fetch session summary and show edit options
	loadBtn = widget.NewButton("Load session", func() {
		idVal, err := strconv.Atoi(idEntry.Text)
		if err != nil {
			outputLabel.SetText("Invalid ID!")
			return
		}
		summary, found := getSessionSummaryByID(db, idVal)
		if !found {
			outputLabel.SetText(summary)
			return
		}
		// show edit options
		idEntry.Hide()
		loadBtn.Hide()
		editTitleBtn.Show()
		editDescBtn.Show()
		editStartBtn.Show()
		editEndBtn.Show()
		editHourlyRateBtn.Show()
		outputLabel.SetText("Choose field to edit: " + summary)
	})

	// buttons to choose which field to edit (show corresponding entry)
	editTitleBtn = widget.NewButton("Edit title", func() {
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editStartBtn.Hide()
		editEndBtn.Hide()
		editHourlyRateBtn.Hide()
		newTitle.Show()
		newTitle.SetPlaceHolder("New title...")
	})

	editDescBtn = widget.NewButton("Edit description", func() {
		editDescBtn.Hide()
		editTitleBtn.Hide()
		editStartBtn.Hide()
		editEndBtn.Hide()
		editHourlyRateBtn.Hide()
		newDesc.Show()
		newDesc.SetPlaceHolder("New description...")
	})

	editStartBtn = widget.NewButton("Edit start time", func() {
		editStartBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editEndBtn.Hide()
		editHourlyRateBtn.Hide()
		newStart.Show()
		newStart.SetPlaceHolder("New start (YYYY-MM-DD HH:MM:SS)")
	})

	editEndBtn = widget.NewButton("Edit end time", func() {
		editEndBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editStartBtn.Hide()
		editHourlyRateBtn.Hide()
		newEnd.Show()
		newEnd.SetPlaceHolder("New end (YYYY-MM-DD HH:MM:SS)")
	})

	editHourlyRateBtn = widget.NewButton("Edit hourly rate", func() {
		editHourlyRateBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editStartBtn.Hide()
		editEndBtn.Hide()
		newHourlyRate.Show()
		newHourlyRate.SetPlaceHolder("New hourly rate (€)")
	})

	// confirm buttons perform the updates and recompute dependent fields (difference, earnings)
	confirmTitleBtn := widget.NewButton("Save title", func() {
		updateQuery = "UPDATE work_sessions SET title = ? WHERE id = ?"
		_, err := db.Exec(updateQuery, newTitle.Text, idEntry.Text)
		if err != nil {
			outputLabel.SetText("Error saving title: " + err.Error())
			return
		}
		outputLabel.SetText("Title updated")
		newTitle.SetText("")
		newTitle.Hide()
		idEntry.Show()
		loadBtn.Show()
	})

	confirmDescBtn := widget.NewButton("Save description", func() {
		updateQuery = "UPDATE work_sessions SET description = ? WHERE id = ?"
		_, err := db.Exec(updateQuery, newDesc.Text, idEntry.Text)
		if err != nil {
			outputLabel.SetText("Error saving description: " + err.Error())
			return
		}
		outputLabel.SetText("Description updated")
		newDesc.SetText("")
		newDesc.Hide()
		idEntry.Show()
		loadBtn.Show()
	})

	// confirm start: need end time and hourly rate from DB to recompute earnings
	confirmStartBtn := widget.NewButton("Save start time", func() {
		getQuery = "SELECT end_time, hourly_rate FROM work_sessions WHERE id = ?"
		row := db.QueryRow(getQuery, idEntry.Text)
		var endStr string
		var hourlyRate float64
		err := row.Scan(&endStr, &hourlyRate)
		if err != nil {
			outputLabel.SetText("Error reading session: " + err.Error())
			return
		}

		endTime, err := time.Parse("2006-01-02 15:04:05", endStr)
		if err != nil {
			outputLabel.SetText("Invalid end time in DB!")
			return
		}
		newStartTime, err := time.Parse("2006-01-02 15:04:05", newStart.Text)
		if err != nil {
			outputLabel.SetText("Invalid start time format!")
			return
		}

		// recompute duration and earnings
		duration := endTime.Sub(newStartTime)
		earnings := math.Round((duration.Hours()*hourlyRate)*100) / 100
		updateQuery = "UPDATE work_sessions SET start_time = ?, difference = ?, earnings = ? WHERE id = ?"
		_, err = db.Exec(updateQuery, newStart.Text, int64(duration.Seconds()), earnings, idEntry.Text)
		if err != nil {
			outputLabel.SetText("Error updating start time: " + err.Error())
			return
		}
		outputLabel.SetText("Start time updated")
		newStart.SetText("")
		newStart.Hide()
		idEntry.Show()
		loadBtn.Show()
	})

	// confirm end: need start time and hourly rate from DB to recompute earnings
	confirmEndBtn := widget.NewButton("Save end time", func() {
		getQuery = "SELECT start_time, hourly_rate FROM work_sessions WHERE id = ?"
		row := db.QueryRow(getQuery, idEntry.Text)
		var startStr string
		var hourlyRate float64
		err := row.Scan(&startStr, &hourlyRate)
		if err != nil {
			outputLabel.SetText("Error reading session: " + err.Error())
			return
		}

		startTime, err := time.Parse("2006-01-02 15:04:05", startStr)
		if err != nil {
			outputLabel.SetText("Invalid start time in DB!")
			return
		}
		newEndTime, err := time.Parse("2006-01-02 15:04:05", newEnd.Text)
		if err != nil {
			outputLabel.SetText("Invalid end time format!")
			return
		}

		duration := newEndTime.Sub(startTime)
		earnings := math.Round((duration.Hours()*hourlyRate)*100) / 100
		updateQuery = "UPDATE work_sessions SET end_time = ?, difference = ?, earnings = ? WHERE id = ?"
		_, err = db.Exec(updateQuery, newEnd.Text, int64(duration.Seconds()), earnings, idEntry.Text)
		if err != nil {
			outputLabel.SetText("Error updating end time: " + err.Error())
			return
		}
		outputLabel.SetText("End time updated")
		newEnd.SetText("")
		newEnd.Hide()
		idEntry.Show()
		loadBtn.Show()
	})

	// confirm rate: read stored difference (seconds), compute earnings with new rate
	confirmHourlyRateBtn := widget.NewButton("Save hourly rate", func() {
		getQuery = "SELECT difference FROM work_sessions WHERE id = ?"
		row := db.QueryRow(getQuery, idEntry.Text)
		var diffSeconds int64
		err := row.Scan(&diffSeconds)
		if err != nil {
			outputLabel.SetText("Error reading difference: " + err.Error())
			return
		}
		duration := time.Duration(diffSeconds) * time.Second
		hours := duration.Hours()
		newRateVal, err := strconv.ParseFloat(newHourlyRate.Text, 64)
		if err != nil {
			outputLabel.SetText("Invalid hourly rate!")
			return
		}
		earnings := math.Round((hours*newRateVal)*100) / 100
		updateQuery = "UPDATE work_sessions SET hourly_rate = ?, earnings = ? WHERE id = ?"
		_, err = db.Exec(updateQuery, newRateVal, earnings, idEntry.Text)
		if err != nil {
			outputLabel.SetText("Error updating hourly rate: " + err.Error())
			return
		}
		outputLabel.SetText("Hourly rate updated")
		newHourlyRate.SetText("")
		newHourlyRate.Hide()
		idEntry.Show()
		loadBtn.Show()
	})

	// hide edit widgets initially so UI starts minimal
	editTitleBtn.Hide()
	editDescBtn.Hide()
	editStartBtn.Hide()
	editEndBtn.Hide()
	editHourlyRateBtn.Hide()
	newTitle.Hide()
	newDesc.Hide()
	newStart.Hide()
	newEnd.Hide()
	newHourlyRate.Hide()
	confirmTitleBtn.Hide()
	confirmDescBtn.Hide()
	confirmStartBtn.Hide()
	confirmEndBtn.Hide()
	confirmHourlyRateBtn.Hide()

	// assemble edit tab layout
	return container.NewVBox(
		widget.NewLabel("Edit session"),
		idEntry,
		loadBtn,
		outputLabel,
		editTitleBtn,
		editDescBtn,
		editStartBtn,
		editEndBtn,
		editHourlyRateBtn,
		newTitle,
		newDesc,
		newStart,
		newEnd,
		newHourlyRate,
		confirmTitleBtn,
		confirmDescBtn,
		confirmStartBtn,
		confirmEndBtn,
		confirmHourlyRateBtn,
	)
}

// createDeleteSessionTab allows deleting a session by ID after confirmation.
func createDeleteSessionTab(db *sql.DB) fyne.CanvasObject {
	var idEntry *widget.Entry
	var loadBtn, confirmBtn *widget.Button
	outputLabel := widget.NewLabel("")
	query := "DELETE FROM work_sessions WHERE id = ?"

	idEntry = widget.NewEntry()
	idEntry.SetPlaceHolder("Enter session ID...")

	// load confirms existence and asks for deletion
	loadBtn = widget.NewButton("Load session", func() {
		idVal, err := strconv.Atoi(idEntry.Text)
		if err != nil {
			outputLabel.SetText("Invalid ID!")
			return
		}
		summary, found := getSessionSummaryByID(db, idVal)
		if !found {
			outputLabel.SetText(summary)
			return
		}
		outputLabel.SetText("Do you really want to delete this session? " + summary)
		loadBtn.Hide()
		confirmBtn.Show()
	})

	// execute deletion when confirmed
	confirmBtn = widget.NewButton("Delete session", func() {
		_, err := db.Exec(query, idEntry.Text)
		if err != nil {
			outputLabel.SetText("Error deleting session: " + err.Error())
			return
		}
		outputLabel.SetText("Session deleted")
		idEntry.SetText("")
		idEntry.Show()
		loadBtn.Show()
		confirmBtn.Hide()
	})

	confirmBtn.Hide()

	return container.NewVBox(
		widget.NewLabel("Delete session"),
		idEntry,
		loadBtn,
		confirmBtn,
		outputLabel,
	)
}

// getAllSessions reads all sessions from the DB and returns them as []Session.
func getAllSessions(db *sql.DB) []Session {
	query := "SELECT id, title, description, start_time, end_time, difference, hourly_rate, earnings, created_by FROM work_sessions ORDER BY end_time DESC"
	rows, err := db.Query(query)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		err := rows.Scan(&s.ID, &s.Title, &s.Description, &s.StartTime, &s.EndTime, &s.Difference, &s.HourlyRate, &s.Earnings, &s.CreatedBy)
		if err != nil {
			panic(err)
		}
		sessions = append(sessions, s)
	}
	return sessions
}

// getSessionSummaryByID returns a printable summary and a boolean indicating if found.
func getSessionSummaryByID(db *sql.DB, id int) (string, bool) {
	query := "SELECT id, uuid, title, description, start_time, end_time, difference, hourly_rate, earnings, created_by FROM work_sessions WHERE id = ?"
	row := db.QueryRow(query, id)

	var (
		sID         int
		sessionUUID string
		title       string
		description string
		startTime   string
		endTime     string
		diffSeconds int64
		hourlyRate  float64
		earnings    float64
		createdBy   string
	)

	err := row.Scan(&sID, &sessionUUID, &title, &description, &startTime, &endTime, &diffSeconds, &hourlyRate, &earnings, &createdBy)
	if err != nil {
		if err == sql.ErrNoRows {
			msg := fmt.Sprintf("No session with ID %d found.", id)
			return msg, false
		}
		panic(err)
	}

	summary := fmt.Sprintf("ID: %d | UUID: %s | Title: %s | Description: %s | %s - %s | Duration: %s | Rate: %.2f€/h | Earnings: %.2f€ | Created by: %s",
		sID, sessionUUID, title, description, startTime, endTime, (time.Duration(diffSeconds) * time.Second).String(), hourlyRate, earnings, createdBy)
	return summary, true
}

// createTable ensures the database schema exists.
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
        hourly_rate REAL,
        earnings REAL,
        created_by TEXT NOT NULL
    );`

	_, err := db.Exec(query)
	if err != nil {
		panic(err)
	}
}

// getDeviceID returns a simple identifier for the current host (used as created_by).
func getDeviceID() string {
	deviceID, err := os.Hostname()
	if err != nil {
		return "Unknown"
	}
	return deviceID
}

// saveSession persists a session. start and end are time.Time so no parsing is required here.
// difference is expected in seconds (int64), hourlyRate and earnings are float64.
func saveSession(db *sql.DB, title string, description string, start time.Time, end time.Time, difference int64, hourlyRate float64, earnings float64) error {
	sessionUUID := uuid.New().String()
	deviceID := getDeviceID()

	query := `INSERT INTO work_sessions (uuid, title, description, start_time, end_time, difference, hourly_rate, earnings, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, sessionUUID, title, description, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), difference, hourlyRate, earnings, deviceID)
	if err != nil {
		return err
	}
	return nil
}
