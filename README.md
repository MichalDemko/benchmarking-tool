# Benchmarking Tool

A flexible HTTP benchmarking tool for load testing APIs with support for multiple endpoints, fixed and ramp modes, and customizable request parameters.

## Features
- Test multiple HTTP endpoints with various methods (GET, POST, DELETE, etc.)
- Support for custom request bodies and headers
- Two load generation modes: `fixed` (constant RPS) and `ramp` (gradually increasing RPS)
- Configurable test duration and request timeouts
- YAML-based configuration for easy setup
- Metrics collection and reporting

## Getting Started

### Prerequisites
- Go 1.22 or newer

### Setup
1. Clone the repository:
   ```sh
   git clone <your-repo-url>
   cd benchmarking-tool
   ```
2. Install dependencies:
   ```sh
   go mod tidy
   ```
3. Copy the example config and edit as needed:
   ```sh
   cp config-example.yaml config.yaml
   # Edit config.yaml to match your endpoints and test parameters
   ```
4. Run the benchmarking tool:
   ```sh
   go run main.go
   ```

## Configuration

Edit `config.yaml` to define your endpoints and test parameters. See `config-example.yaml` for a template and documentation of available options.

### Modes

#### fixed
Sends a fixed number of requests per second (RPS) to the defined endpoints. Endpoints are selected in a round-robin fashion, distributing the load evenly across all specified endpoints. This mode is fully implemented and recommended for most use cases.

#### ramp
(TODO: Not fully implemented yet)

Gradually increases the number of requests per second over time, starting from an initial RPS and ramping up by a specified increment at regular intervals until a maximum RPS is reached. This mode is intended for stress testing and observing system behavior under increasing load.

## TODO
- [ ] Implement full support for ramp mode (dynamic RPS adjustment)
- [ ] Add more detailed metrics and reporting (latency percentiles, error rates)
- [ ] Support for additional HTTP methods and authentication schemes
- [ ] CLI flags for overriding config values
- [ ] Dockerfile for containerized runs
- [ ] CI/CD integration and automated tests

## Contributing
Contributions are welcome! Please open issues or submit pull requests for new features, bug fixes, or improvements.

## License
MIT License
