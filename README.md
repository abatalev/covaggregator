# JaCoCo Coverage Aggregator

A system for collecting, aggregating, and publishing Java
application code coverage data at runtime.

## Documentation

- [docs/architecture.md](docs/architecture.md) – overview of
  how it works and the architecture.
- [docs/plan.md](docs/plan.md) – detailed implementation
  plan.

## Brief Description

This project is a Go application that periodically polls JVM
instances (microservices) using `jacococli dump`, collects
coverage data (`.exec` files), aggregates it by versions and
services, generates reports in HTML/XML/CSV formats, and
publishes them via a built-in web server (Chi). Dynamic
configuration management through REST API and Prometheus
metrics export are supported.

## Quick Start

1. Clone the repository.
2. Edit `config.yaml` (see example in plan.md).
3. Build the Docker image with `make docker` (or build the
   binary with `make build`).
4. Run the container:

   ```bash
   docker run -v $(pwd)/config.yaml:/app/config.yaml -p 8080:8080 jacoco-aggregator:latest
   ```

5. Open `http://localhost:8080` to view reports.

![Web UI](docs/screenshot.png)

## Example Usage

To demonstrate the aggregator's functionality, a sample Java
microservice (`demo-service-flyway-pg`) is included.

### Option 1: Docker image with pre-loaded data

Build and run the pre-built Docker image with example data:

1. Build the Docker image with example data:

   ```bash
   make build-example
   ```

2. Run the container:

   ```bash
   docker run -p 8080:8080 jacoco-aggregator-example
   ```

### Option 2: Running example services locally

Run example services with docker compose and connect
aggregator:

1. Start the example services (clones repo, builds, and
   starts containers):

   ```bash
   make start-example-stand
   ```

2. Start the aggregator:

   ```bash
   make start-example
   ```

3. Open `http://localhost:8080` to view reports.

4. When done, stop the example services:

   ```bash
   make stop-example-stand
   ```

### Available Make commands

- `make start-example-stand` – clone example repository,
  build services and start docker compose
- `make start-example` – start the aggregator with example
  configuration
- `make stop-example-stand` – stop docker compose services

## Development

The project uses Makefile for common tasks:

- `make test` – run unit and integration tests (using
  `testify`).
- `make lint` – run code linter (`golangci-lint`).
- `make coverage` – generate coverage report for the
  aggregator itself (target – 80% or higher).
- `make build` – build the binary to `bin/aggregator`.
- `make docker` – build the Docker image
  `jacoco-aggregator:latest`.

To run tests with coverage, execute `make coverage`; the
report will be saved to `coverage.out` and printed to the
terminal.

Before committing, it is recommended to run `make lint` and
`make test`.

## Requirements

**Important:** Microservices must be started with the JaCoCo
Java agent for coverage data collection to work:

```bash
java -javaagent:jacocoagent.jar=destfile=jacoco.exec,address=0.0.0.0 -jar your-service.jar
```

Port 6300 must be open for the aggregator to connect to the
JaCoCo agent.

- Docker
- Java 8+ (for `jacococli`)
- Go 1.21+ (for building from source)
- Maven (for building the example via `make build-example`)

## License

MIT
