package main

import (
	"fmt"
	"time"
)

func main() {
	timer()
}

func timer() {
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
	elapsed := end.Sub(start).Round(time.Second) // Rundet auf volle Sekunden
	fmt.Printf("elapsed time: %v\n", elapsed)
}
