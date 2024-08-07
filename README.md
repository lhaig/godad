# godad

Godad is a Go application that fetches and displays dad jokes from the [icanhazdadjoke.com](https://icanhazdadjoke.com/) API. It stores previously fetched jokes in a SQLite database to ensure you always get a fresh joke.

## Features

- Fetches jokes from the icanhazdadjoke.com API
- Stores jokes in a SQLite database
- Ensures each joke is unique (not previously fetched)
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

- `dbdir`: Directory to store the SQLite database (default: current directory)

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

```
./bin/godad --dbdir /path/to/your/database/directory
```

## Usage

To run the application and get a dad joke:

```
make run
```

This will fetch a new joke from the API, store it in the database, and display it. If the joke has been seen before, it will fetch another one until it finds a new joke.

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

## Acknowledgments

- [icanhazdadjoke.com](https://icanhazdadjoke.com/) for providing the dad jokes API
- All contributors to this project