package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	_ "modernc.org/sqlite"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// Session represents one work session stored in the database.
// Difference is stored as seconds (int64) and Earnings as float64.
type Session struct {
	ID          int
	uuid        string
	Title       string
	Description string
	StartTime   string
	EndTime     string
	startUnix   int64
	endUnix     int64
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
	userConfigDir, _ := os.UserConfigDir()
	dbPath := filepath.Join(userConfigDir, "TaskTracker", "taskTracker.db")
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	db, err := sql.Open("sqlite", dbPath)
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
		container.NewTabItem("Export", exportSessions(db)),
	)

	myWindow.SetContent(tabs)
	myWindow.ShowAndRun()
}

// createTimerTab builds the timer UI where user can start/stop and save a session.
// The timer calculates duration (time.Duration) and earnings before saving.
func createTimerTab(db *sql.DB) fyne.CanvasObject {
	var start, end time.Time
	var duration time.Duration
	var ticker *time.Ticker
	var tickerQuit chan struct{}
	var startBtn, stopBtn, saveBtn, setRateBtn *widget.Button
	var titleEntry, descEntry, hourlyRateEntry *widget.Entry

	// status + bindings for thread-safe live updates
	statusLabel := widget.NewLabel("Timer ready")
	elapsedData := binding.NewString()
	elapsedLabel := widget.NewLabelWithData(elapsedData)
	_ = elapsedData.Set("00:00:00")

	earningsData := binding.NewString()
	earningsLabel := widget.NewLabelWithData(earningsData)
	_ = earningsData.Set("0.00€")

	// show current rate
	rateDisplay := widget.NewLabel("Rate: -")
	var currentRate float64
	var rateSet bool

	// create entries
	titleEntry = widget.NewEntry()
	descEntry = widget.NewEntry()
	hourlyRateEntry = widget.NewEntry()

	// button to lock in the hourly rate before starting
	setRateBtn = widget.NewButton("Set rate", func() {
		r, err := strconv.ParseFloat(hourlyRateEntry.Text, 64)
		if err != nil {
			statusLabel.SetText("Invalid hourly rate!")
			return
		}
		currentRate = r
		rateSet = true
		rateDisplay.SetText(fmt.Sprintf("Rate: %.2f€/h", currentRate))
		hourlyRateEntry.Hide()
		setRateBtn.Hide()
	})

	// start button: requires a set rate (or tries to parse one)
	startBtn = widget.NewButton("Start timer", func() {
		if !rateSet {
			// try to parse rate on start if not set
			r, err := strconv.ParseFloat(hourlyRateEntry.Text, 64)
			if err != nil {
				statusLabel.SetText("Set a valid hourly rate first!")
				return
			}
			currentRate = r
			rateSet = true
			rateDisplay.SetText(fmt.Sprintf("Rate: %.2f€/h", currentRate))
			hourlyRateEntry.Hide()
			setRateBtn.Hide()
		}

		start = time.Now()
		statusLabel.SetText("Timer running...")
		startBtn.Hide()
		stopBtn.Show()
		elapsedLabel.Show()
		earningsLabel.Show()

		// stop any previous ticker
		if ticker != nil {
			ticker.Stop()
		}
		if tickerQuit != nil {
			close(tickerQuit)
		}

		ticker = time.NewTicker(time.Second)
		tickerQuit = make(chan struct{})

		// ticker goroutine updates elapsed and earnings via binding
		go func(s time.Time, t *time.Ticker, q chan struct{}) {
			for {
				select {
				case <-t.C:
					el := time.Since(s)
					h := int(el.Hours())
					m := int(el.Minutes()) % 60
					s := int(el.Seconds()) % 60
					_ = elapsedData.Set(fmt.Sprintf("%02d:%02d:%02d", h, m, s))

					earned := math.Round((el.Hours()*currentRate)*100) / 100
					_ = earningsData.Set(fmt.Sprintf("%.2f€", earned))
				case <-q:
					return
				}
			}
		}(start, ticker, tickerQuit)
	})

	// stop button: stop ticker, compute final duration and show save inputs
	stopBtn = widget.NewButton("Stop timer", func() {
		// stop ticker
		if ticker != nil {
			ticker.Stop()
			ticker = nil
		}
		if tickerQuit != nil {
			close(tickerQuit)
			tickerQuit = nil
		}

		end = time.Now()
		duration = end.Sub(start)
		statusLabel.SetText("Timer stopped")
		stopBtn.Hide()

		// show inputs to save session
		titleEntry.Show()
		descEntry.Show()
		hourlyRateEntry.Show()
		setRateBtn.Show()
		saveBtn.Show()

		// final values
		h := int(duration.Hours())
		m := int(duration.Minutes()) % 60
		se := int(duration.Seconds()) % 60
		_ = elapsedData.Set(fmt.Sprintf("%02d:%02d:%02d", h, m, se))

		finalEarned := math.Round((duration.Hours()*currentRate)*100) / 100
		_ = earningsData.Set(fmt.Sprintf("%.2f€", finalEarned))
	})

	// save button: persist and reset timer + earnings
	saveBtn = widget.NewButton("Save session", func() {
		// hide inputs and show start again
		titleEntry.Hide()
		descEntry.Hide()
		hourlyRateEntry.Hide()
		setRateBtn.Hide()
		saveBtn.Hide()
		startBtn.Show()

		// allow rate to be re-entered next session
		rateSet = false
		rateDisplay.SetText("Rate: -")

		// if user changed rate entry before save, prefer parsed value; otherwise use currentRate
		if hourlyRateEntry.Text != "" {
			if r, err := strconv.ParseFloat(hourlyRateEntry.Text, 64); err == nil {
				currentRate = r
			}
		}

		hours := duration.Hours()
		earnings := math.Round((hours*currentRate)*100) / 100

		// save session (start/end passed as time.Time)
		err := saveSession(db, titleEntry.Text, descEntry.Text, start, end, int64(duration.Seconds()), currentRate, earnings)
		if err != nil {
			statusLabel.SetText("Error saving session: " + err.Error())
			return
		}

		// feedback and clear
		statusLabel.SetText(fmt.Sprintf("Session '%s' saved. Duration: %s. Earnings: %.2f€", titleEntry.Text, duration.String(), earnings))
		titleEntry.SetText("")
		descEntry.SetText("")
		hourlyRateEntry.SetText("")

		// reset live displays
		_ = elapsedData.Set("00:00:00")
		_ = earningsData.Set("0.00€")
		elapsedLabel.Hide()
		earningsLabel.Hide()
		hourlyRateEntry.Show()
		setRateBtn.Show()
	})

	// initial visibility
	titleEntry.Hide()
	descEntry.Hide()
	hourlyRateEntry.SetPlaceHolder("Hourly rate (€)")
	hourlyRateEntry.Show() // let user enter rate before start
	setRateBtn.Show()
	saveBtn.Hide()
	stopBtn.Hide()
	elapsedLabel.Hide()
	earningsLabel.Hide()

	// placeholders and text config
	titleEntry.SetPlaceHolder("Enter title...")
	descEntry.SetPlaceHolder("Description (optional)")
	descEntry.MultiLine = true

	// layout: status, live displays, controls, inputs
	return container.NewVBox(
		statusLabel,
		container.NewHBox(widget.NewLabel("Elapsed: "), elapsedLabel, widget.NewLabel("  Earned: "), earningsLabel, rateDisplay),
		container.NewVBox(hourlyRateEntry, setRateBtn, startBtn, stopBtn),
		widget.NewSeparator(),
		titleEntry,
		descEntry,
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
				widget.NewLabel("ID: "+strconv.Itoa(s.ID)),
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
	var confirmTitleBtn, confirmDescBtn, confirmStartBtn, confirmEndBtn, confirmHourlyRateBtn *widget.Button
	var cancelBtn *widget.Button
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
		idEntry.SetText("")
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
		confirmTitleBtn.Show()
		cancelBtn.Show()
	})

	editDescBtn = widget.NewButton("Edit description", func() {
		editDescBtn.Hide()
		editTitleBtn.Hide()
		editStartBtn.Hide()
		editEndBtn.Hide()
		editHourlyRateBtn.Hide()
		newDesc.Show()
		newDesc.SetPlaceHolder("New description...")
		confirmDescBtn.Show()
		cancelBtn.Show()
	})

	editStartBtn = widget.NewButton("Edit start time", func() {
		editStartBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editEndBtn.Hide()
		editHourlyRateBtn.Hide()
		newStart.Show()
		newStart.SetPlaceHolder("New start (YYYY-MM-DD HH:MM:SS)")
		confirmStartBtn.Show()
		cancelBtn.Show()
	})

	editEndBtn = widget.NewButton("Edit end time", func() {
		editEndBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editStartBtn.Hide()
		editHourlyRateBtn.Hide()
		newEnd.Show()
		newEnd.SetPlaceHolder("New end (YYYY-MM-DD HH:MM:SS)")
		confirmEndBtn.Show()
		cancelBtn.Show()
	})

	editHourlyRateBtn = widget.NewButton("Edit hourly rate", func() {
		editHourlyRateBtn.Hide()
		editTitleBtn.Hide()
		editDescBtn.Hide()
		editStartBtn.Hide()
		editEndBtn.Hide()
		newHourlyRate.Show()
		newHourlyRate.SetPlaceHolder("New hourly rate (€)")
		confirmHourlyRateBtn.Show()
		cancelBtn.Show()
	})

	// confirm buttons perform the updates and recompute dependent fields (difference, earnings)
	confirmTitleBtn = widget.NewButton("Save title", func() {
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
		confirmTitleBtn.Hide()
	})

	confirmDescBtn = widget.NewButton("Save description", func() {
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
		confirmDescBtn.Hide()
	})

	// confirm start: need end time and hourly rate from DB to recompute earnings
	confirmStartBtn = widget.NewButton("Save start time", func() {
		getQuery = "SELECT end_time, hourly_rate FROM work_sessions WHERE id = ?"
		row := db.QueryRow(getQuery, idEntry.Text)
		var endStr string
		var hourlyRate float64
		err := row.Scan(&endStr, &hourlyRate)
		if err != nil {
			outputLabel.SetText("Error reading session: " + err.Error())
			return
		}

		endTime, err := time.Parse("2006-01-02 15:04", endStr)
		if err != nil {
			outputLabel.SetText("Invalid end time in DB!")
			return
		}
		newStartTime, err := time.Parse("2006-01-02 15:04", newStart.Text)
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
		confirmStartBtn.Hide()
	})

	// confirm end: need start time and hourly rate from DB to recompute earnings
	confirmEndBtn = widget.NewButton("Save end time", func() {
		getQuery = "SELECT start_time, hourly_rate FROM work_sessions WHERE id = ?"
		row := db.QueryRow(getQuery, idEntry.Text)
		var startStr string
		var hourlyRate float64
		err := row.Scan(&startStr, &hourlyRate)
		if err != nil {
			outputLabel.SetText("Error reading session: " + err.Error())
			return
		}

		startTime, err := time.Parse("2006-01-02 15:04", startStr)
		if err != nil {
			outputLabel.SetText("Invalid start time in DB!")
			return
		}
		newEndTime, err := time.Parse("2006-01-02 15:04", newEnd.Text)
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
		confirmEndBtn.Hide()
	})

	// confirm rate: read stored difference (seconds), compute earnings with new rate
	confirmHourlyRateBtn = widget.NewButton("Save hourly rate", func() {
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
		confirmHourlyRateBtn.Hide()
	})

	cancelBtn = widget.NewButton("Cancel", func() {
		// hide all edit widgets and show id entry + load button
		newTitle.Hide()
		newDesc.Hide()
		newStart.Hide()
		newEnd.Hide()
		newHourlyRate.Hide()
		newTitle.SetText("")
		newDesc.SetText("")
		newStart.SetText("")
		newEnd.SetText("")
		newHourlyRate.SetText("")
		confirmTitleBtn.Hide()
		confirmDescBtn.Hide()
		confirmStartBtn.Hide()
		confirmEndBtn.Hide()
		confirmHourlyRateBtn.Hide()
		cancelBtn.Hide()
		idEntry.Show()
		loadBtn.Show()
		outputLabel.SetText("Edit cancelled")
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
	cancelBtn.Hide()

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
		cancelBtn,
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
	query := "SELECT id, uuid, title, description, start_time, end_time, start_unix, end_unix, difference, hourly_rate, earnings, created_by FROM work_sessions ORDER BY end_time DESC"
	rows, err := db.Query(query)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		err := rows.Scan(&s.ID, &s.uuid, &s.Title, &s.Description, &s.StartTime, &s.EndTime, &s.endUnix, &s.startUnix, &s.Difference, &s.HourlyRate, &s.Earnings, &s.CreatedBy)
		if err != nil {
			panic(err)
		}
		sessions = append(sessions, s)
	}
	return sessions
}

// getSessionSummaryByID returns a printable summary and a boolean indicating if found.
func getSessionSummaryByID(db *sql.DB, id int) (string, bool) {
	query := "SELECT id, uuid, title, description, start_time, end_time, start_unix, end_unix, difference, hourly_rate, earnings, created_by FROM work_sessions WHERE id = ?"
	row := db.QueryRow(query, id)

	var (
		sID         int
		sessionUUID string
		title       string
		description string
		startTime   string
		endTime     string
		startUnix   int64
		endUnix     int64
		diffSeconds int64
		hourlyRate  float64
		earnings    float64
		createdBy   string
	)

	err := row.Scan(&sID, &sessionUUID, &title, &description, &startTime, &endTime, &startUnix, &endUnix, &diffSeconds, &hourlyRate, &earnings, &createdBy)
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
		start_unix INTEGER,
		end_unix INTEGER,
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

	query := `INSERT INTO work_sessions (uuid, title, description, start_time, end_time, start_unix, end_unix, difference, hourly_rate, earnings, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query, sessionUUID, title, description, start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"), start.Unix(), end.Unix(), difference, hourlyRate, earnings, deviceID)
	if err != nil {
		return err
	}
	return nil
}

// parseExportTime parses input like "2006-01-02 15:04" and returns time in local location.
func parseExportTime(input string) (time.Time, error) {
	layout := "2006-01-02 15:04"
	input = strings.TrimSpace(input)
	if input == "" {
		return time.Time{}, fmt.Errorf("empty input")
	}
	// use ParseInLocation so local timezone is applied
	return time.ParseInLocation(layout, input, time.Local)
}

func exportSessions(db *sql.DB) fyne.CanvasObject {
	statusLabel := widget.NewLabel("")
	startExport := widget.NewEntry()
	endExport := widget.NewEntry()
	var exportCSVBtn, exportXLSXBtn, exportSortCSVBtn, exportSortXLSXBtn, sortBtn, cancelBtn *widget.Button

	sortBtn = widget.NewButton("Sort by time range", func() {
		startExport.Show()
		endExport.Show()
		exportSortCSVBtn.Show()
		exportSortXLSXBtn.Show()
		sortBtn.Hide()
		exportCSVBtn.Hide()
		exportXLSXBtn.Hide()
	})

	cancelBtn = widget.NewButton("Cancel", func() {
		startExport.Hide()
		endExport.Hide()
		exportSortCSVBtn.Hide()
		exportSortXLSXBtn.Hide()
		sortBtn.Show()
		exportCSVBtn.Show()
		exportXLSXBtn.Show()
		startExport.SetText("")
		endExport.SetText("")
		statusLabel.SetText("")
	})

	exportCSVBtn = widget.NewButton("Export to CSV", func() {
		filename := "export_" + time.Now().Format("2006-01-02_15-04-05") + ".csv"
		parent := fyne.CurrentApp().Driver().AllWindows()[0]
		fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if w == nil {
				return // user cancelled
			}
			defer w.Close()

			// write BOM + CSV
			_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
			writer := csv.NewWriter(w)
			writer.Comma = ';'

			header := []string{"Title", "Description", "Start Time", "End Time", "Duration", "Hourly Rate (€)", "Earnings (€)"}
			if err := writer.Write(header); err != nil {
				statusLabel.SetText("Error writing header: " + err.Error())
				return
			}

			sessions := getAllSessions(db)
			for _, s := range sessions {
				row := []string{s.Title, s.Description, s.StartTime, s.EndTime, (time.Duration(s.Difference) * time.Second).String(), fmt.Sprintf("%.2f", s.HourlyRate), fmt.Sprintf("%.2f", s.Earnings)}
				if err := writer.Write(row); err != nil {
					statusLabel.SetText("Error writing row: " + err.Error())
					return
				}
			}
			writer.Flush()
			if err := writer.Error(); err != nil {
				statusLabel.SetText("Error finalizing CSV: " + err.Error())
				return
			}
			statusLabel.SetText(fmt.Sprintf("Exported %d sessions to %s", len(sessions), w.URI().Name()))
		}, parent)
		fd.SetFileName(filename)
		fd.Show()
	})

	// replace exportSortCSVBtn handler (uses parsed time range)
	exportSortCSVBtn = widget.NewButton("Export to CSV", func() {
		startT, err := parseExportTime(startExport.Text)
		if err != nil {
			statusLabel.SetText("Invalid start time. Use format: 2006-01-02 15:04")
			return
		}
		endT, err := parseExportTime(endExport.Text)
		if err != nil {
			statusLabel.SetText("Invalid end time. Use format: 2006-01-02 15:04")
			return
		}
		if endT.Before(startT) {
			statusLabel.SetText("End time must be after start time")
			return
		}

		filename := "export_" + time.Now().Format("2006-01-02_15-04-05") + ".csv"
		parent := fyne.CurrentApp().Driver().AllWindows()[0]
		fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if w == nil {
				return
			}
			defer w.Close()

			_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
			writer := csv.NewWriter(w)
			writer.Comma = ';'
			header := []string{"Title", "Description", "Start Time", "End Time", "Duration", "Hourly Rate (€)", "Earnings (€)"}
			if err := writer.Write(header); err != nil {
				statusLabel.SetText("Error writing header: " + err.Error())
				return
			}

			sessions := getAllSessions(db)
			exported := 0
			for _, s := range sessions {
				if s.startUnix < startT.Unix() || s.endUnix > endT.Unix() {
					continue
				}
				row := []string{s.Title, s.Description, s.StartTime, s.EndTime, (time.Duration(s.Difference) * time.Second).String(), fmt.Sprintf("%.2f", s.HourlyRate), fmt.Sprintf("%.2f", s.Earnings)}
				if err := writer.Write(row); err != nil {
					statusLabel.SetText("Error writing row: " + err.Error())
					return
				}
				exported++
			}
			writer.Flush()
			if err := writer.Error(); err != nil {
				statusLabel.SetText("Error finalizing CSV: " + err.Error())
				return
			}
			statusLabel.SetText(fmt.Sprintf("Exported %d sessions to %s", exported, w.URI().Name()))
		}, parent)
		fd.SetFileName(filename)
		fd.Show()
	})

	// replace exportXLSXBtn handler
	exportXLSXBtn = widget.NewButton("Export to XLSX", func() {
		filename := "export_" + time.Now().Format("2006-01-02_15-04-05") + ".xlsx"
		parent := fyne.CurrentApp().Driver().AllWindows()[0]
		fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if w == nil {
				return
			}
			defer w.Close()

			sessions := getAllSessions(db)
			f := excelize.NewFile()
			defer f.Close()
			sheet := "Sheet1"
			headers := []string{"Title", "Description", "Start Time", "End Time", "Duration", "Hourly Rate", "Earnings"}
			for i, h := range headers {
				cell := string(rune('A'+i)) + "1"
				f.SetCellValue(sheet, cell, h)
			}
			for rowIdx, s := range sessions {
				row := rowIdx + 2
				rowStr := strconv.Itoa(row)
				f.SetCellValue(sheet, "A"+rowStr, s.Title)
				f.SetCellValue(sheet, "B"+rowStr, s.Description)
				f.SetCellValue(sheet, "C"+rowStr, s.StartTime)
				f.SetCellValue(sheet, "D"+rowStr, s.EndTime)
				f.SetCellValue(sheet, "E"+rowStr, (time.Duration(s.Difference) * time.Second).String())
				f.SetCellValue(sheet, "F"+rowStr, s.HourlyRate)
				f.SetCellValue(sheet, "G"+rowStr, s.Earnings)
			}
			// write excel file to writer
			if _, err := f.WriteTo(w); err != nil {
				statusLabel.SetText("Error writing XLSX: " + err.Error())
				return
			}
			statusLabel.SetText(fmt.Sprintf("Exported %d sessions to %s", len(sessions), w.URI().Name()))
		}, parent)
		fd.SetFileName(filename)
		fd.Show()
	})

	// replace exportSortXLSXBtn handler
	exportSortXLSXBtn = widget.NewButton("Export to XLSX", func() {
		startT, err := parseExportTime(startExport.Text)
		if err != nil {
			statusLabel.SetText("Invalid start time. Use format: 2006-01-02 15:04")
			return
		}
		endT, err := parseExportTime(endExport.Text)
		if err != nil {
			statusLabel.SetText("Invalid end time. Use format: 2006-01-02 15:04")
			return
		}
		if endT.Before(startT) {
			statusLabel.SetText("End time must be after start time")
			return
		}

		filename := "export_" + time.Now().Format("2006-01-02_15-04-05") + ".xlsx"
		parent := fyne.CurrentApp().Driver().AllWindows()[0]
		fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if w == nil {
				return
			}
			defer w.Close()

			sessions := getAllSessions(db)
			f := excelize.NewFile()
			defer f.Close()
			sheet := "Sheet1"
			headers := []string{"Title", "Description", "Start Time", "End Time", "Duration", "Hourly Rate", "Earnings"}
			for i, h := range headers {
				cell := string(rune('A'+i)) + "1"
				f.SetCellValue(sheet, cell, h)
			}
			rowNo := 2
			exported := 0
			for _, s := range sessions {
				if s.startUnix < startT.Unix() || s.endUnix > endT.Unix() {
					continue
				}
				rowStr := strconv.Itoa(rowNo)
				f.SetCellValue(sheet, "A"+rowStr, s.Title)
				f.SetCellValue(sheet, "B"+rowStr, s.Description)
				f.SetCellValue(sheet, "C"+rowStr, s.StartTime)
				f.SetCellValue(sheet, "D"+rowStr, s.EndTime)
				f.SetCellValue(sheet, "E"+rowStr, (time.Duration(s.Difference) * time.Second).String())
				f.SetCellValue(sheet, "F"+rowStr, s.HourlyRate)
				f.SetCellValue(sheet, "G"+rowStr, s.Earnings)
				rowNo++
				exported++
			}
			if _, err := f.WriteTo(w); err != nil {
				statusLabel.SetText("Error writing XLSX: " + err.Error())
				return
			}
			statusLabel.SetText(fmt.Sprintf("Exported %d sessions to %s", exported, w.URI().Name()))
		}, parent)
		fd.SetFileName(filename)
		fd.Show()
	})

	exportSortCSVBtn.Hide()
	exportSortXLSXBtn.Hide()
	startExport.Hide()
	endExport.Hide()
	cancelBtn.Hide()
	exportXLSXBtn.Show()
	exportCSVBtn.Show()
	startExport.SetPlaceHolder("Format: 2006-01-02 15:04")
	endExport.SetPlaceHolder("Format: 2006-01-02 15:04")

	return container.NewVBox(
		statusLabel,
		exportCSVBtn,
		exportXLSXBtn,
		sortBtn,
		startExport,
		endExport,
		exportSortCSVBtn,
		exportSortXLSXBtn,
		cancelBtn,
	)
}
