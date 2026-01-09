# godad

Godad is a Go application that fetches and displays dad jokes in multiple languages. It supports English jokes from the [icanhazdadjoke.com](https://icanhazdadjoke.com/) API and German jokes (Flachwitze) from a curated markdown source. It stores previously fetched jokes in a SQLite database to ensure you always get a fresh joke.

## Features

- Fetches jokes from multiple sources:
  - English: [icanhazdadjoke.com](https://icanhazdadjoke.com/) API
  - German: [Flachwitze collection](https://github.com/derphilipp/Flachwitze)
- Multi-language support (English and German)
- Stores jokes in a SQLite database with language tagging
- Ensures each joke is unique per language (not previously fetched)
- Configurable database location
- Supports environment variables, .env file, and command-line flags for configuration

## Requirements

- Go 1.21 or later
- SQLite3

## Installation

1. Clone the repository:
   ```
   git clone https://github.com/lhaig/godad.git
   cd godad
   ```

2. Install dependencies:
   ```
   go mod download
   ```

3. Build the application:
   ```
   make build
   ```

## Configuration

You can configure the application using one of the following methods (in order of precedence):

1. Command-line flags
2. Environment variables
3. .env file
4. Default values

### Configuration Options

- `dbdir`: Directory to store the SQLite database (default: `~/.godad`)
- `lang`: Language for jokes - `en` for English or `de` for German (default: `en`)

### Using a .env file

Create a `.env` file in the root directory of the project with the following content:

```
DBDIR=/path/to/your/database/directory
```

### Using environment variables

Set the `DBDIR` environment variable:

```
export DBDIR=/path/to/your/database/directory
```

### Using command-line flags

```bash
# Get an English joke (default)
./bin/godad

# Get a German joke
./bin/godad --lang de

# Specify custom database directory
./bin/godad --dbdir /path/to/your/database/directory

# Combine options
./bin/godad --lang de --dbdir /path/to/your/database/directory
```

## Usage

To run the application and get a dad joke:

```bash
# Get an English joke
make run

# Or run directly
./bin/godad
```

To get a German joke:

```bash
./bin/godad --lang de
```

### How it Works

The application will:
1. Fetch a joke from the appropriate source based on the selected language
2. Check if the joke has been seen before (per language)
3. Store the joke in the database with its language tag
4. Display the joke to you
5. If the joke was already seen, it will retry up to 5 times to find a fresh joke
6. If no fresh jokes are available, it will show a random joke from the database in the selected language

## Development

### Running Tests

To run the tests:

```
make test
```

### Running Linter

To run the linter:

```
make lint
```

To automatically fix linter issues (where possible):

```
make lint-fix
```

### Testing Coverage

To generate a test coverage report:

```
make test-coverage
```

This will create a `coverage.html` file that you can open in your browser to view the coverage report.

## Docker

### Building Docker Image

To build a Docker image:

```
make docker-build
```

### Running with Docker

To run the application in a Docker container:

```
make docker-run
```

Note: When using Docker, you might need to modify the Dockerfile to include your .env file or pass environment variables to the container for custom configuration.

## CI/CD

This project uses GitHub Actions for continuous integration and deployment. The workflow includes:

- Linting
- Testing
- Building
- Releasing (on tag push)

## License

This project is licensed under the Mozilla Public License 2.0. See the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## Database

The application uses SQLite to store jokes. The database schema includes:

- `id`: Unique identifier for each joke
- `joke`: The joke text
- `language`: Language code (`en` or `de`)
- `created_at`: Timestamp when the joke was first fetched

Jokes are tracked per language, so the same joke text can exist in both English and German databases without conflict.

### Migration

If you're upgrading from an older version without language support, the application will automatically:
- Add the `language` column to your existing database
- Set all existing jokes to `language='en'` (English)
- Continue working without any data loss

## Acknowledgments

- [icanhazdadjoke.com](https://icanhazdadjoke.com/) for providing the English dad jokes API
- [Flachwitze by derphilipp](https://github.com/derphilipp/Flachwitze) for the German jokes collection
- All contributors to this project