package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestMain(m *testing.M) {
	// Set up test configuration
	viper.Set("dbdir", ":memory:")

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
	code := m.Run()

	// Exit
	os.Exit(code)
}

func TestGetFreshJoke(t *testing.T) {
	// Clear the database before the test
	_, err := db.Exec("DELETE FROM jokes")
	if err != nil {
		t.Fatalf("Failed to clear the database: %v", err)
	}

	// Create a mock server
	jokeResponses := []string{
		`{"id": "1", "joke": "This is the first joke", "status": 200}`,
		`{"id": "2", "joke": "This is the second joke", "status": 200}`,
		`{"id": "3", "joke": "This is the third joke", "status": 200}`,
		`{"id": "4", "joke": "This is the fourth joke", "status": 200}`,
		`{"id": "5", "joke": "This is the fifth joke", "status": 200}`,
	}
	currentJoke := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request has the correct headers
		if r.Header.Get("User-Agent") != "https://github.com/lhaig/godad" {
			t.Errorf("Expected User-Agent header to be 'https://github.com/lhaig/godad', got %s", r.Header.Get("User-Agent"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept header to be 'application/json', got %s", r.Header.Get("Accept"))
		}

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
	setAPIURL(server.URL) // Update this line to pass the server URL
	defer setAPIURL(originalAPIURL) // Update this line to pass the original API URL

	// Test getting fresh jokes
	expectedJokes := []string{
		"This is the first joke",
		"This is the second joke",
		"This is the third joke",
	}

	for i, expected := range expectedJokes {
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

	if len(jokes) != len(expectedJokes) {
		t.Errorf("Expected %d jokes in database, got %d", len(expectedJokes), len(jokes))
	}

	for i, joke := range jokes {
		if i < len(expectedJokes) && joke != expectedJokes[i] {
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
	setAPIURL(server.URL) // Update this line to pass the server URL
	defer setAPIURL(originalAPIURL) // Update this line to pass the original API URL

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
	setAPIURL(server.URL) // Update this line to pass the server URL
	defer setAPIURL(originalAPIURL) // Update this line to pass the original API URL

	// Call the getJoke function
	_, err := getJoke()

	// Check if there was an error
	if err == nil {
		t.Errorf("getJoke() did not return an error for invalid JSON")
	}
}

func TestInitConfig(t *testing.T) {
	// Save current environment and defer its restoration
	oldEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, pair := range oldEnv {
			parts := strings.SplitN(pair, "=", 2)
			os.Setenv(parts[0], parts[1])
		}
	}()

	// Set a mock home directory for testing
	mockHomeDir := "/mock/home"
	os.Setenv("HOME", mockHomeDir)

	defaultDBDir := filepath.Join(mockHomeDir, ".godad")

	// Test cases
	testCases := []struct {
		name        string
		envVars     map[string]string
		args        []string
		expectedDir string
		expectedLang string
	}{
		{
			name:        "Default",
			envVars:     map[string]string{},
			args:        []string{},
			expectedDir: defaultDBDir,
			expectedLang: "en",
		},
		{
			name:        "EnvVar",
			envVars:     map[string]string{"DBDIR": "/env/path"},
			args:        []string{},
			expectedDir: "/env/path",
			expectedLang: "en",
		},
		{
			name:        "Flag",
			envVars:     map[string]string{},
			args:        []string{"--dbdir", "/flag/path"},
			expectedDir: "/flag/path",
			expectedLang: "en",
		},
		{
			name:        "FlagOverridesEnvVar",
			envVars:     map[string]string{"DBDIR": "/env/path"},
			args:        []string{"--dbdir", "/flag/path"},
			expectedDir: "/flag/path",
			expectedLang: "en",
		},
		{
			name:        "LanguageFlag",
			envVars:     map[string]string{},
			args:        []string{"--lang", "de"},
			expectedDir: defaultDBDir,
			expectedLang: "de",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset viper and flags
			viper.Reset()
			pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)

			// Set environment variables
			os.Clearenv()
			os.Setenv("HOME", mockHomeDir) // Ensure HOME is always set
			for k, v := range tc.envVars {
				os.Setenv(k, v)
			}

			// Set command line args
			os.Args = append([]string{"cmd"}, tc.args...)

			// Run initConfig
			err := initConfig()
			if err != nil {
				t.Fatalf("initConfig() returned an error: %v", err)
			}

			// Check result
			if dir := viper.GetString("dbdir"); dir != tc.expectedDir {
				t.Errorf("Expected dbdir to be %s, got %s", tc.expectedDir, dir)
			}

			// Check language
			if lang := language; lang != tc.expectedLang {
				t.Errorf("Expected language to be %s, got %s", tc.expectedLang, lang)
			}
		})
	}
}
