
# AGENTS.md

This file provides guidelines for agentic coding agents operating in this repository.

## Build/Lint/Test Commands

### Build
- `make build`: Build the binary (includes frontend build)
- `make frontend-build`: Build frontend static files
- `make clean`: Clean up generated files

### Lint
- `make lint`: Run golangci-lint (enables: gofmt, govet, staticcheck, gosimple, unused, errcheck, ineffassign, typecheck, revive)

### Test
- `make test`: Run all unit and integration tests with verbose output
- `make test-race`: Run tests with race detector enabled
- `go test ./... -v`: Run all tests with verbose output
- `go test ./path/to/package -v`: Run tests for a specific package
- `go test -run TestName ./... -v`: Run a single test by name
- `go test -run TestName ./path/to/package -v`: Run a single test in a specific package

### Coverage
- `make coverage`: Generate coverage report and check against 80% threshold

### Docker
- `make docker`: Build Docker image from Dockerfile.base

### Frontend
- `make frontend-install`: Install frontend dependencies using npm ci
- `make frontend-build`: Build frontend static files
- `make frontend-clean`: Remove frontend build artifacts

### Development
- `make download-jacococli`: Download JaCoCo CLI jar for report merging
- `make clone-example`: Clone example repository (demo-service-flyway-pg) into example/
- `make build-example`: Build example services and create Docker image with example data
- `make start-example`: Start the aggregator with config.example.local.yaml and ./data storage

## Code Style Guidelines

### General
- Use Go language idioms and standard library when possible
- Keep functions small and focused on single responsibility
- Use meaningful variable and function names that describe intent

### Imports
- Group imports in the following order:
  1. Standard library packages (e.g., "fmt", "errors", "io")
  2. Third-party packages (e.g., "github.com/stretchr/testify", "github.com/jackc/pgx")
  3. Local packages (e.g., "internal/...", "pkg/...")
- Sort imports alphabetically within each group
- Use blank line between groups

### Formatting
- Use `gofmt` for formatting code
- Use `golines` for line length (120 characters max)
- Use tabs for indentation, not spaces

### Types
- Use named types for public APIs (exported structs, interfaces)
- Use anonymous structs for private data
- Use interfaces to define behavior, not concrete types
- Avoid embedding types unless intentional composition is needed

### Naming Conventions
- Use camelCase for variable and function names
- Use PascalCase for type names and exported functions
- Use ALL_CAPS for constants
- Use short but descriptive names (e.g., `req`, `resp`, `cfg`)
- Avoid redundant prefixes (e.g., `userUser` -> `user`)

### Error Handling
- Prefer returning errors over panicking
- Use `errors.New` for simple error messages
- Use `fmt.Errorf` with %w for wrapping errors
- Check errors immediately and handle or return
- Avoid ignoring errors with `_`

### Context Usage
- Pass context.Context as first argument to functions that make external calls
- Use `context.Background()` for top-level operations
- Use `context.WithTimeout()` for operations with deadlines

### Resource Management
- Use defer for cleanup (e.g., file.Close(), mutex.Unlock())
- Close resources in reverse order of creation
- Use sync.Pool for frequently allocated objects

### Testing
- Use table-driven tests for complex logic with multiple test cases
- Use `t.Parallel()` for parallel tests when tests don't share state
- Use `testify` for assertions (require or assert packages)
- Name test files `*_test.go` and co-locate with implementation
- Use descriptive test names: `TestFunctionName_Scenario_ExpectedResult`
- Use subtests for variations within a test function

### Frontend (React/TypeScript)
- Use functional components with hooks
- Use TypeScript for type safety
- Use meaningful component and variable names
- Keep components small and focused

### Configuration
- Use YAML for configuration files
- Provide sensible defaults
- Validate configuration at startup

## Cursor Rules

### General
- Use `gofmt` for formatting
- Use `golines` for line length (120 characters)
- Follow the code style guidelines in this file

### Testing
- Use table-driven tests for complex logic
- Use `t.Parallel()` for parallel tests
- Use `testify` for assertions

## Copilot Instructions

### General
- Follow the code style guidelines above
- Use `gofmt` for formatting
- Use `golines` for line length (120 characters)

### Testing
- Use table-driven tests for complex logic
- Use `t.Parallel()` for parallel tests
- Use `testify` for assertions
