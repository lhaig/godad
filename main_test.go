// Copyright (c) 2024 Lance Haig
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMain(m *testing.M) {
	// Set up test database
	var err error
	db, err = sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Create table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS jokes (
		id TEXT PRIMARY KEY,
		joke TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		panic(err)
	}

	// Run tests
	os.Exit(m.Run())
}

func TestGetFreshJoke(t *testing.T) {
	// Create a mock server
	jokeResponses := []string{
		`{"id": "1", "joke": "This is the first joke", "status": 200}`,
		`{"id": "2", "joke": "This is the second joke", "status": 200}`,
		`{"id": "1", "joke": "This is the first joke", "status": 200}`,
		`{"id": "3", "joke": "This is the third joke", "status": 200}`,
	}
	currentJoke := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(jokeResponses[currentJoke]))
		if err != nil {
			t.Errorf("Error writing response: %v", err)
		}
		currentJoke = (currentJoke + 1) % len(jokeResponses)
	}))
	defer server.Close()

	// Set the apiURL to our mock server URL
	originalAPIURL := apiURL
	setAPIURL(server.URL)
	defer setAPIURL(originalAPIURL)

	// Test getting fresh jokes
	for i, expected := range []string{
		"This is the first joke",
		"This is the second joke",
		"This is the third joke",
	} {
		joke, err := getFreshJoke()
		if err != nil {
			t.Errorf("getFreshJoke() returned an error: %v", err)
		}
		if joke != expected {
			t.Errorf("getFreshJoke() returned %s, want %s (iteration %d)", joke, expected, i)
		}
	}

	// Check database contents
	rows, err := db.Query("SELECT joke FROM jokes ORDER BY created_at")
	if err != nil {
		t.Fatalf("Error querying database: %v", err)
	}
	defer rows.Close()

	var jokes []string
	for rows.Next() {
		var joke string
		if err := rows.Scan(&joke); err != nil {
			t.Fatalf("Error scanning row: %v", err)
		}
		jokes = append(jokes, joke)
	}

	expectedJokes := []string{
		"This is the first joke",
		"This is the second joke",
		"This is the third joke",
	}

	if len(jokes) != len(expectedJokes) {
		t.Errorf("Expected %d jokes in database, got %d", len(expectedJokes), len(jokes))
	}

	for i, joke := range jokes {
		if joke != expectedJokes[i] {
			t.Errorf("Joke %d in database is %s, want %s", i, joke, expectedJokes[i])
		}
	}
}

func TestGetJokeAPIError(t *testing.T) {
	// Create a mock server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Set the apiURL to our mock server URL
	originalAPIURL := apiURL
	setAPIURL(server.URL)
	defer setAPIURL(originalAPIURL)

	// Call the getJoke function
	_, err := getJoke()

	// Check if there was an error
	if err == nil {
		t.Errorf("getJoke() did not return an error for API failure")
	}
}

func TestGetJokeInvalidJSON(t *testing.T) {
	// Create a mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"id": "R7UfaahVfFd", "joke": "This is an invalid JSON`))
		if err != nil {
			t.Errorf("Error writing response: %v", err)
		}
	}))
	defer server.Close()

	// Set the apiURL to our mock server URL
	originalAPIURL := apiURL
	setAPIURL(server.URL)
	defer setAPIURL(originalAPIURL)

	// Call the getJoke function
	_, err := getJoke()

	// Check if there was an error
	if err == nil {
		t.Errorf("getJoke() did not return an error for invalid JSON")
	}
}
