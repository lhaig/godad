package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	// Set up test configuration
	viper.Set("dbdir", ":memory:")

	// Initialize the database
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Create table with language and shown columns
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS jokes (
		id INTEGER PRIMARY KEY,
		joke TEXT NOT NULL,
		language TEXT NOT NULL DEFAULT 'en',
		shown BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		panic(err)
	}

	// Create metadata table for tests
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS metadata (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		panic(err)
	}

	// Set default language for tests
	language = "en"

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
	apiURL = server.URL // Directly set the apiURL to the mock server URL
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
	apiURL = server.URL // Directly set the apiURL to the mock server URL

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
	apiURL = server.URL // Directly set the apiURL to the mock server URL

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
		name         string
		envVars      map[string]string
		args         []string
		expectedDir  string
		expectedLang string
	}{
		{
			name:         "Default",
			envVars:      map[string]string{},
			args:         []string{},
			expectedDir:  defaultDBDir,
			expectedLang: "en",
		},
		{
			name:         "EnvVar",
			envVars:      map[string]string{"DBDIR": "/env/path"},
			args:         []string{},
			expectedDir:  "/env/path",
			expectedLang: "en",
		},
		{
			name:         "Flag",
			envVars:      map[string]string{},
			args:         []string{"--dbdir", "/flag/path"},
			expectedDir:  "/flag/path",
			expectedLang: "en",
		},
		{
			name:         "FlagOverridesEnvVar",
			envVars:      map[string]string{"DBDIR": "/env/path"},
			args:         []string{"--dbdir", "/flag/path"},
			expectedDir:  "/flag/path",
			expectedLang: "en",
		},
		{
			name:         "LanguageFlag",
			envVars:      map[string]string{},
			args:         []string{"--lang", "de"},
			expectedDir:  defaultDBDir,
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

func TestSetAPIURL(t *testing.T) {
	testCases := []struct {
		name        string
		language    string
		expectedURL string
	}{
		{
			name:        "English language",
			language:    "en",
			expectedURL: enAPIURL,
		},
		{
			name:        "German language",
			language:    "de",
			expectedURL: deAPIURL,
		},
		{
			name:        "Default (empty string)",
			language:    "",
			expectedURL: enAPIURL,
		},
		{
			name:        "Unknown language defaults to English",
			language:    "fr",
			expectedURL: enAPIURL,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			language = tc.language
			setAPIURL()
			if apiURL != tc.expectedURL {
				t.Errorf("Expected apiURL to be %s, got %s", tc.expectedURL, apiURL)
			}
		})
	}
}

func TestGetRandomJokeFromDB(t *testing.T) {
	// Save original language and restore after test
	origLang := language
	defer func() { language = origLang }()

	// Ensure language is set to English for this test
	language = "en"

	// Clear the database before the test
	_, err := db.Exec("DELETE FROM jokes")
	if err != nil {
		t.Fatalf("Failed to clear the database: %v", err)
	}

	// Test with empty database
	t.Run("Empty database", func(t *testing.T) {
		_, err := getRandomJokeFromDB()
		if err == nil {
			t.Error("Expected error when database is empty, got nil")
		}
	})

	// Insert test jokes
	testJokes := []string{
		"Why don't scientists trust atoms? Because they make up everything!",
		"What do you call a fake noodle? An impasta!",
		"Why did the scarecrow win an award? He was outstanding in his field!",
	}

	for _, joke := range testJokes {
		_, err := db.Exec("INSERT INTO jokes (joke, language) VALUES (?, ?)", joke, "en")
		if err != nil {
			t.Fatalf("Failed to insert test joke: %v", err)
		}
	}

	// Test retrieving a random joke
	t.Run("Retrieve random joke", func(t *testing.T) {
		joke, err := getRandomJokeFromDB()
		if err != nil {
			t.Fatalf("getRandomJokeFromDB() returned an error: %v", err)
		}

		// Check that the joke is one of our test jokes
		found := false
		for _, testJoke := range testJokes {
			if joke == testJoke {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Retrieved joke %q is not in the test set", joke)
		}
	})

	// Test that multiple calls can return different jokes (probabilistic test)
	t.Run("Randomness check", func(t *testing.T) {
		if len(testJokes) < 2 {
			t.Skip("Need at least 2 jokes for randomness test")
		}

		jokes := make(map[string]bool)
		attempts := 20

		for i := 0; i < attempts; i++ {
			joke, err := getRandomJokeFromDB()
			if err != nil {
				t.Fatalf("getRandomJokeFromDB() returned an error: %v", err)
			}
			jokes[joke] = true
		}

		// With 3 jokes and 20 attempts, we should get at least 2 different jokes
		if len(jokes) < 2 {
			t.Logf("Warning: Only got %d unique joke(s) in %d attempts, expected at least 2", len(jokes), attempts)
		}
	})
}

func TestExtractJokesFromMarkdown(t *testing.T) {
	testCases := []struct {
		name     string
		markdown string
		expected []string
	}{
		{
			name: "Valid markdown with jokes",
			markdown: `# Jokes
- Why did the chicken cross the road?
- What do you call a bear with no teeth?
- How does a penguin build its house?`,
			expected: []string{
				"Why did the chicken cross the road?",
				"What do you call a bear with no teeth?",
				"How does a penguin build its house?",
			},
		},
		{
			name:     "Empty markdown",
			markdown: "",
			expected: nil,
		},
		{
			name: "Markdown without jokes",
			markdown: `# Title
This is some text
Another line`,
			expected: nil,
		},
		{
			name: "Mixed content",
			markdown: `# Jokes
Some intro text
- First joke
Not a joke
- Second joke`,
			expected: []string{
				"First joke",
				"Second joke",
			},
		},
		{
			name: "Jokes with extra spaces",
			markdown: `- Joke with spaces
-  Joke with leading space`,
			expected: []string{
				"Joke with spaces",
				" Joke with leading space",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractJokesFromMarkdown(tc.markdown)

			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d jokes, got %d", len(tc.expected), len(result))
			}

			for i, joke := range result {
				if i < len(tc.expected) && joke != tc.expected[i] {
					t.Errorf("Joke %d: expected %q, got %q", i, tc.expected[i], joke)
				}
			}
		})
	}
}

func TestGetFreshJokeRetryLogic(t *testing.T) {
	// Clear the database before the test
	_, err := db.Exec("DELETE FROM jokes")
	if err != nil {
		t.Fatalf("Failed to clear the database: %v", err)
	}

	// Create a mock server that always returns the same joke
	sameJoke := `{"id": "1", "joke": "Always the same joke", "status": 200}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(sameJoke))
		if err != nil {
			t.Errorf("Error writing response: %v", err)
		}
	}))
	defer server.Close()

	apiURL = server.URL

	// First call should succeed
	t.Run("First joke succeeds", func(t *testing.T) {
		joke, err := getFreshJoke()
		if err != nil {
			t.Fatalf("First getFreshJoke() should succeed: %v", err)
		}
		if joke != "Always the same joke" {
			t.Errorf("Expected 'Always the same joke', got %q", joke)
		}
	})

	// Second call should fail after max retries (all jokes are duplicates)
	t.Run("Duplicate jokes fail after retries", func(t *testing.T) {
		joke, err := getFreshJoke()
		if err == nil {
			t.Errorf("Expected error after max retries, but got joke: %q", joke)
		}
		if err != nil && !strings.Contains(err.Error(), "could not find a new joke") {
			t.Errorf("Expected 'could not find a new joke' error, got: %v", err)
		}
	})
}

func TestGetFreshJokeWithDuplicates(t *testing.T) {
	// Save original language and restore after test
	origLang := language
	defer func() { language = origLang }()

	// Ensure language is set to English for this test
	language = "en"

	// Clear the database before the test
	_, err := db.Exec("DELETE FROM jokes")
	if err != nil {
		t.Fatalf("Failed to clear the database: %v", err)
	}

	// Create a mock server that returns duplicate then unique joke
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		var response string
		callCount++
		if callCount == 1 {
			response = `{"id": "1", "joke": "Duplicate joke", "status": 200}`
		} else {
			response = `{"id": "2", "joke": "Unique joke", "status": 200}`
		}

		_, err := w.Write([]byte(response))
		if err != nil {
			t.Errorf("Error writing response: %v", err)
		}
	}))
	defer server.Close()

	apiURL = server.URL

	// Insert the duplicate joke
	_, err = db.Exec("INSERT INTO jokes (joke, language) VALUES (?, ?)", "Duplicate joke", "en")
	if err != nil {
		t.Fatalf("Failed to insert duplicate joke: %v", err)
	}

	// Should retry and eventually get the unique joke
	joke, err := getFreshJoke()
	if err != nil {
		t.Fatalf("getFreshJoke() should succeed after retries: %v", err)
	}
	if joke != "Unique joke" {
		t.Errorf("Expected 'Unique joke', got %q", joke)
	}
}

func TestGetJokeMarkdown(t *testing.T) {
	// Create a mock server that returns markdown
	markdownContent := `# Flachwitze

Some intro text here.

- Warum hat der Weihnachtsmann einen so großen Sack? Weil er nur einmal im Jahr kommt.
- Was ist grün und steht vor der Tür? Ein Klopfsalat.
- Wie nennt man einen Bumerang, der nicht zurückkommt? Stock.

More text here.
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(markdownContent))
		if err != nil {
			t.Errorf("Error writing response: %v", err)
		}
	}))
	defer server.Close()

	apiURL = server.URL

	// Call getJoke - it should extract and return one of the jokes
	joke, err := getJoke()
	if err != nil {
		t.Fatalf("getJoke() returned an error: %v", err)
	}

	// Check that we got one of the expected jokes
	expectedJokes := []string{
		"Warum hat der Weihnachtsmann einen so großen Sack? Weil er nur einmal im Jahr kommt.",
		"Was ist grün und steht vor der Tür? Ein Klopfsalat.",
		"Wie nennt man einen Bumerang, der nicht zurückkommt? Stock.",
	}

	found := false
	for _, expected := range expectedJokes {
		if joke == expected {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("getJoke() returned unexpected joke: %q", joke)
	}
}

func TestGetJokeMarkdownNoJokes(t *testing.T) {
	// Create a mock server that returns markdown without jokes
	markdownContent := `# Title

This is some text without any joke markers.
Just regular paragraphs.
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(markdownContent))
		if err != nil {
			t.Errorf("Error writing response: %v", err)
		}
	}))
	defer server.Close()

	apiURL = server.URL

	// Call getJoke - it should return an error
	_, err := getJoke()
	if err == nil {
		t.Error("getJoke() should return error when no jokes found in markdown")
	}
	if err != nil && !strings.Contains(err.Error(), "no jokes found") {
		t.Errorf("Expected 'no jokes found' error, got: %v", err)
	}
}

func TestGetFreshJokeGerman(t *testing.T) {
	// Save original language and restore after test
	origLang := language
	origDeAPIURL := deAPIURL
	defer func() {
		language = origLang
		deAPIURL = origDeAPIURL
	}()

	// Set language to German
	language = "de"

	// Clear the database before the test
	_, err := db.Exec("DELETE FROM jokes")
	if err != nil {
		t.Fatalf("Failed to clear the database: %v", err)
	}
	_, err = db.Exec("DELETE FROM metadata")
	if err != nil {
		t.Fatalf("Failed to clear metadata: %v", err)
	}

	// Create a mock server that returns German jokes as markdown
	markdownContent := `# Flachwitze
- Erster deutscher Witz
- Zweiter deutscher Witz
- Dritter deutscher Witz
`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(markdownContent))
		if err != nil {
			t.Errorf("Error writing response: %v", err)
		}
	}))
	defer server.Close()

	deAPIURL = server.URL

	// Sync German jokes
	err = syncGermanJokes()
	if err != nil {
		t.Fatalf("Failed to sync German jokes: %v", err)
	}

	// Verify all 3 jokes are in database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM jokes WHERE language = 'de'").Scan(&count)
	if err != nil {
		t.Fatalf("Error querying database: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 German jokes in database after sync, got %d", count)
	}

	// Get jokes - should get unique jokes until all are shown
	seenJokes := make(map[string]bool)
	for i := 0; i < 3; i++ {
		joke, err := getFreshJoke()
		if err != nil {
			t.Fatalf("getFreshJoke() returned an error on iteration %d: %v", i, err)
		}
		seenJokes[joke] = true
	}

	// Should have seen all 3 unique jokes
	if len(seenJokes) != 3 {
		t.Errorf("Expected to see 3 unique jokes, got %d", len(seenJokes))
	}

	// Next joke should reset and start over
	joke, err := getFreshJoke()
	if err != nil {
		t.Fatalf("getFreshJoke() should work after reset: %v", err)
	}
	if !seenJokes[joke] {
		t.Errorf("Expected to get one of the previous jokes after reset, got new joke: %s", joke)
	}
}
