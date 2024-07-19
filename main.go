// Copyright (c) 2024 Lance Haig
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	apiURL = "https://icanhazdadjoke.com/"
	mu     sync.RWMutex
	db     *sql.DB
)

// ResponseObject represents the structure of the API response
type ResponseObject struct {
	ID     string `json:"id"`
	Joke   string `json:"joke"`
	Status int    `json:"status"`
}

func main() {
	// Initialize logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Initialize database
	var err error
	db, err = sql.Open("sqlite3", "./jokes.db")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()

	// Create table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS jokes (
		id TEXT PRIMARY KEY,
		joke TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create table")
	}

	// Get joke
	joke, err := getFreshJoke()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get joke")
		os.Exit(1)
	}

	// Print joke
	fmt.Println(joke)
}

// getFreshJoke fetches a joke that hasn't been used before
func getFreshJoke() (string, error) {
	for {
		joke, err := getJoke()
		if err != nil {
			return "", err
		}

		// Check if joke exists in database
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM jokes WHERE joke = ?", joke).Scan(&count)
		if err != nil {
			return "", fmt.Errorf("error checking joke existence: %w", err)
		}

		if count == 0 {
			// Joke doesn't exist, insert it and return
			_, err = db.Exec("INSERT INTO jokes (joke) VALUES (?)", joke)
			if err != nil {
				return "", fmt.Errorf("error inserting joke: %w", err)
			}
			return joke, nil
		}

		// If joke exists, try again
		log.Info().Msg("Joke already exists, fetching another one")
	}
}

// getJoke fetches a joke from the API
func getJoke() (string, error) {
	// Create a new HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	mu.RLock()
	url := apiURL
	mu.RUnlock()

	// Create a new request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", "https://github.com/lhaig/godad")
	req.Header.Set("Accept", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	// Parse the JSON response
	var responseObject ResponseObject
	if err := json.Unmarshal(body, &responseObject); err != nil {
		return "", fmt.Errorf("error parsing JSON: %w", err)
	}

	return responseObject.Joke, nil
}

// setAPIURL allows changing the API URL (used for testing)
func setAPIURL(url string) {
	mu.Lock()
	apiURL = url
	mu.Unlock()
}
