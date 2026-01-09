package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	_ "modernc.org/sqlite"
)

var (
	enAPIURL = "https://icanhazdadjoke.com/"
	deAPIURL = "https://raw.githubusercontent.com/derphilipp/Flachwitze/main/README.md"
	mu       sync.RWMutex
	db       *sql.DB
	apiURL   string // Define apiURL variable
	language string // Define language variable
)

// ResponseObject represents the structure of the API response
type ResponseObject struct {
	ID     string `json:"id"`
	Joke   string `json:"joke"`
	Status int    `json:"status"`
}

func main() {
	// Initialize configuration
	if err := initConfig(); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize configuration")
	}

	// Initialize logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Get database directory from configuration
	dbDir := viper.GetString("dbdir")

	// Ensure the database directory exists
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		log.Fatal().Err(err).Msg("Failed to create database directory")
	}

	// Initialize database
	dbPath := filepath.Join(dbDir, "jokesdev.db")
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()

	log.Info().Str("path", dbPath).Msg("SQLite Database initialized")

	// Create table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS jokes (
		id INTEGER PRIMARY KEY,
		joke TEXT NOT NULL,
		language TEXT NOT NULL DEFAULT 'en',
		shown BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create table")
	}

	// Create metadata table for tracking sync times
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS metadata (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create metadata table")
	}

	// Migrate existing data: add language column if it doesn't exist
	_, err = db.Exec(`ALTER TABLE jokes ADD COLUMN language TEXT NOT NULL DEFAULT 'en'`)
	if err != nil {
		// Column already exists or other error - check if it's the "duplicate column" error
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Warn().Err(err).Msg("Migration warning (this is normal if column already exists)")
		}
	}

	// Migrate existing data: add shown column if it doesn't exist
	_, err = db.Exec(`ALTER TABLE jokes ADD COLUMN shown BOOLEAN NOT NULL DEFAULT 0`)
	if err != nil {
		// Column already exists or other error - check if it's the "duplicate column" error
		if !strings.Contains(err.Error(), "duplicate column") {
			log.Warn().Err(err).Msg("Migration warning (this is normal if column already exists)")
		}
	}

	// For German jokes, sync from markdown if needed
	if language == "de" {
		if err := syncGermanJokesIfNeeded(); err != nil {
			log.Warn().Err(err).Msg("Failed to sync German jokes, will use existing database")
		}
	}

	// Get joke
	joke, err := getFreshJoke()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get a fresh joke")
		// Here you might want to implement a fallback strategy,
		// such as returning a random joke from the database
		randomJoke, err := getRandomJokeFromDB()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to get a random joke from the database")
		}
		joke = randomJoke
	}

	// Print joke
	fmt.Println(joke)
}

func initConfig() error {
	homedrive, err := os.UserHomeDir()
	if err != nil {
		log.Err(err)
	}
	dblocation := filepath.Join(homedrive, ".godad")
	// Set default values
	viper.SetDefault("dbdir", dblocation)

	// Read from .env file
	viper.SetConfigName("config")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AddConfigPath(filepath.Join(homedrive, ".godad"))
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("error reading config file: %w", err)
		}
		// It's okay if the config file is not found, we'll use defaults and flags
	}
	fmt.Println("Using config file:", viper.ConfigFileUsed())
	// Read from environment variables
	viper.AutomaticEnv()

	// Define and parse flags
	pflag.String("dbdir", viper.GetString("dbdir"), "Directory to store the SQLite database")
	pflag.StringVar(&language, "lang", "en", "Language for jokes (en or de)")
	pflag.Parse()

	// Bind flags to viper
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return fmt.Errorf("error binding flags: %w", err)
	}

	// Set the API URL based on the selected language
	setAPIURL()

	return nil
}

// setAPIURL sets the apiURL variable based on the selected language
func setAPIURL() {
	mu.Lock()
	defer mu.Unlock()
	if language == "de" {
		apiURL = deAPIURL
	} else {
		apiURL = enAPIURL
	}
}

// getFreshJoke fetches a joke that hasn't been used before
func getFreshJoke() (string, error) {
	// German jokes: get from database (already synced)
	if language == "de" {
		return getUnshownGermanJoke()
	}

	// English jokes: fetch from API
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		joke, err := getJoke()
		if err != nil {
			return "", fmt.Errorf("error fetching joke from API: %w", err)
		}

		// Check if joke exists in database for this language
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM jokes WHERE joke = ? AND language = ?", joke, language).Scan(&count)
		if err != nil {
			return "", fmt.Errorf("error checking joke existence: %w", err)
		}

		if count == 0 {
			// Joke doesn't exist, insert it and mark as shown
			_, err = db.Exec("INSERT INTO jokes (joke, language, shown) VALUES (?, ?, 1)", joke, language)
			if err != nil {
				return "", fmt.Errorf("error inserting joke: %w", err)
			}
			return joke, nil
		}

		// If joke exists, log and try again
		log.Info().Msg("Joke already exists, fetching another one")
	}

	// If we've reached this point, we couldn't find a new joke after maxRetries
	return "", fmt.Errorf("could not find a new joke after %d attempts", maxRetries)
}

// getUnshownGermanJoke gets a random unshown German joke from the database and marks it as shown
func getUnshownGermanJoke() (string, error) {
	// Try to get an unshown joke
	var joke string
	var id int64
	err := db.QueryRow("SELECT id, joke FROM jokes WHERE language = 'de' AND shown = 0 ORDER BY RANDOM() LIMIT 1").Scan(&id, &joke)

	if err == sql.ErrNoRows {
		// All jokes have been shown, reset them
		log.Info().Msg("All German jokes have been shown! Resetting...")
		_, err := db.Exec("UPDATE jokes SET shown = 0 WHERE language = 'de'")
		if err != nil {
			return "", fmt.Errorf("error resetting shown status: %w", err)
		}

		// Try again
		err = db.QueryRow("SELECT id, joke FROM jokes WHERE language = 'de' AND shown = 0 ORDER BY RANDOM() LIMIT 1").Scan(&id, &joke)
		if err != nil {
			return "", fmt.Errorf("error getting joke after reset: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("error getting unshown joke: %w", err)
	}

	// Mark the joke as shown
	_, err = db.Exec("UPDATE jokes SET shown = 1 WHERE id = ?", id)
	if err != nil {
		return "", fmt.Errorf("error marking joke as shown: %w", err)
	}

	return joke, nil
}

// getJoke fetches a joke from the API
func getJoke() (string, error) {
	// Create a new HTTP client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Try to get a joke from the selected API
	if apiURL == "" {
		return "", fmt.Errorf("no valid API URL provided")
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("User-Agent", "https://github.com/lhaig/godad")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %w", err)
	}

	// Check if the response is JSON
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var responseObject ResponseObject
		if err := json.Unmarshal(body, &responseObject); err != nil {
			return "", fmt.Errorf("error parsing JSON: %w", err)
		}
		return responseObject.Joke, nil
	}

	// If not JSON, treat as markdown (for German jokes)
	jokes := extractJokesFromMarkdown(string(body))
	if len(jokes) == 0 {
		return "", fmt.Errorf("no jokes found in markdown content")
	}

	// Return a random joke from the markdown
	return jokes[randInt(len(jokes))], nil
}

// randInt returns a random integer between 0 and n-1
func randInt(n int) int {
	return rand.IntN(n) //nolint:gosec // cryptographic randomness not needed for joke selection
}

// extractJokesFromMarkdown extracts jokes from the markdown content
func extractJokesFromMarkdown(markdown string) []string {
	var jokes []string
	lines := strings.Split(markdown, "\n")
	for _, line := range lines {
		// Assuming jokes are listed with a specific marker, e.g., "- "
		if strings.HasPrefix(line, "- ") {
			joke := strings.TrimPrefix(line, "- ")
			jokes = append(jokes, joke)
		}
	}
	return jokes
}

// getRandomJokeFromDB retrieves a random joke from the database for the current language
func getRandomJokeFromDB() (string, error) {
	var joke string
	err := db.QueryRow("SELECT joke FROM jokes WHERE language = ? ORDER BY RANDOM() LIMIT 1", language).Scan(&joke)
	if err != nil {
		return "", fmt.Errorf("error getting random joke from database: %w", err)
	}
	return joke, nil
}

// syncGermanJokesIfNeeded checks if German jokes need to be synced and syncs them if necessary
func syncGermanJokesIfNeeded() error {
	const syncInterval = 7 * 24 * time.Hour // 7 days

	// Check when we last synced
	var lastSyncStr string
	err := db.QueryRow("SELECT value FROM metadata WHERE key = 'german_jokes_last_sync'").Scan(&lastSyncStr)

	needsSync := false
	if err == sql.ErrNoRows {
		// Never synced before
		needsSync = true
	} else if err != nil {
		return fmt.Errorf("error checking last sync time: %w", err)
	} else {
		// Parse the last sync time
		lastSync, err := time.Parse(time.RFC3339, lastSyncStr)
		if err != nil {
			log.Warn().Err(err).Msg("Invalid last sync time, will re-sync")
			needsSync = true
		} else if time.Since(lastSync) > syncInterval {
			needsSync = true
		}
	}

	if needsSync {
		log.Info().Msg("Syncing German jokes from markdown...")
		return syncGermanJokes()
	}

	return nil
}

// syncGermanJokes downloads the German jokes markdown and adds new jokes to the database
func syncGermanJokes() error {
	// Download the markdown content
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", deAPIURL, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("User-Agent", "https://github.com/lhaig/godad")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error downloading German jokes: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	// Extract jokes from markdown
	jokes := extractJokesFromMarkdown(string(body))
	if len(jokes) == 0 {
		return fmt.Errorf("no jokes found in markdown")
	}

	// Insert new jokes (skip duplicates)
	newJokesCount := 0
	for _, joke := range jokes {
		// Check if joke already exists
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM jokes WHERE joke = ? AND language = 'de'", joke).Scan(&count)
		if err != nil {
			log.Warn().Err(err).Str("joke", joke).Msg("Error checking joke existence")
			continue
		}

		if count == 0 {
			// Insert new joke
			_, err = db.Exec("INSERT INTO jokes (joke, language, shown) VALUES (?, 'de', 0)", joke)
			if err != nil {
				log.Warn().Err(err).Str("joke", joke).Msg("Error inserting joke")
				continue
			}
			newJokesCount++
		}
	}

	// Update last sync time
	now := time.Now().Format(time.RFC3339)
	_, err = db.Exec(`INSERT OR REPLACE INTO metadata (key, value, updated_at) VALUES ('german_jokes_last_sync', ?, CURRENT_TIMESTAMP)`, now)
	if err != nil {
		return fmt.Errorf("error updating last sync time: %w", err)
	}

	log.Info().Int("new_jokes", newJokesCount).Int("total_jokes", len(jokes)).Msg("German jokes synced successfully")
	return nil
}
