# Benchmarking Tool

A flexible HTTP benchmarking tool for load testing APIs with support for multiple endpoints, dynamic parameter generation, and comprehensive reporting.

## Features
- ✅ **Test multiple HTTP endpoints** with various methods (GET, POST, PUT, DELETE, etc.)
- ✅ **Advanced parameter generation system** with multiple generator types
- ✅ **Support for path parameters, query parameters, and request bodies**
- ✅ **Dynamic parameter value generation** (random integers, formatted strings, choices, etc.)
- ✅ **Flexible endpoint selection strategies** (round-robin, weighted, random)
- ✅ **Fixed RPS load generation mode**
- ✅ **Configurable test duration and request timeouts**
- ✅ **YAML-based configuration** for easy setup
- ✅ **Comprehensive metrics collection and detailed reporting**
- ✅ **Static string values and generator references** with clear `$ref` syntax

## Getting Started

### Prerequisites
- Go 1.22 or newer

### Quick Start
1. Clone and build:
   ```sh
   git clone <your-repo-url>
   cd benchmarking-tool
   go mod tidy              # Install dependencies
   go build -o benchmarking-tool
   ```

2. Run with the included example:
   ```sh
   ./benchmarking-tool config-examples/simple-example.yml
   ```

3. Create your own configuration:
   ```sh
   cp config-examples/simple-example.yml my-test.yml
   # Edit my-test.yml to match your API
   ./benchmarking-tool my-test.yml
   ```

## Configuration

The tool uses YAML configuration files to define endpoints, parameter generation, and test execution settings. See `config-examples/simple-example.yml` for a working example.

### Configuration Structure

```yaml
# Base URLs for your API
baseUrls:
  - "https://api.example.com"
  - "https://backup-api.example.com"

# Test execution settings
execution:
  mode: "fixed"                # Only "fixed" mode is currently supported
  durationSeconds: 60          # Test duration in seconds
  requestTimeoutMs: 5000       # Individual request timeout in milliseconds
  requestsPerSecond: 50        # Target requests per second

# Named parameter generators (reusable across endpoints)
parameterGenerators:
  user_id:
    type: "formattedInt"       # Generate formatted string from integer
    min: 1000
    max: 9999
    format: "user_{}"          # Results in "user_1234", "user_5678", etc.
  
  age:
    type: "randomInt"          # Generate plain integer
    min: 18
    max: 65

# API endpoints to test
endpoints:
  get_user:
    path: "/api/v1/users/{user_id}"
    method: "GET"
    pathParameters:
      user_id:
        $ref: "user_id"        # Reference to named generator
    queryParameters:
      fields: "id,name,email"  # Static string value
      format: "json"           # Another static string

# How to select endpoints during testing
endpointSelection:
  strategy: "weighted"         # "roundRobin", "weighted", or "random"
  weights:
    get_user: 0.6
    create_user: 0.4
```

### Parameter Generators

Parameter generators create dynamic values for requests. They can be defined as named generators (reusable) or inline within endpoint definitions.

#### Parameter Value Types

The configuration supports three ways to specify parameter values:

1. **Static Strings**: Plain string values are treated as static
   ```yaml
   queryParameters:
     format: "json"            # Always returns "json"
     version: "v1"             # Always returns "v1"
   ```

2. **Generator References**: Use `$ref` to reference named generators
   ```yaml
   pathParameters:
     user_id:
       $ref: "user_id"         # References parameterGenerators.user_id
   ```

3. **Inline Generator Definitions**: Full generator objects
   ```yaml
   bodyParameters:
     age:
       type: "randomInt"
       min: 18
       max: 65
   ```

#### Generator Types

All generator types listed below are ✅ **fully implemented** and ready to use:

##### `randomInt` ✅
Generates random integers within a specified range.
```yaml
type: "randomInt"
min: 1              # Minimum value (inclusive)
max: 100            # Maximum value (inclusive)
```
**Output**: Plain integer (e.g., `42`, `87`)

##### `formattedInt` ✅
Generates formatted strings using random integers.
```yaml
type: "formattedInt"
min: 1000
max: 9999
format: "user_{}"   # {} is replaced with the generated number
```
**Output**: Formatted string (e.g., `"user_1234"`, `"user_5678"`)

##### `static` ✅
Returns a fixed value.
```yaml
type: "static"
value: "fixed-value"
```
**Output**: The exact value specified

##### `choice` ✅
Randomly selects from a list of values, optionally with weights.
```yaml
type: "choice"
values: ["active", "inactive", "pending"]
weights: [0.5, 0.3, 0.2]  # Optional: probability weights
```
**Output**: One of the specified values

##### `randomString` ✅
Generates random strings with specified length and character set.
```yaml
type: "randomString"
length: 10
charset: "alphanumeric"  # "alpha", "alpha_lower", "alpha_upper", "numeric", "alphanumeric", "hex"
```
**Output**: Random string (e.g., `"a3B7xK9mP2"`)

##### `template` ✅
Generates values using templates with embedded parameters.
```yaml
type: "template"
template: "Hello {{name}}, you are {{age}} years old"
parameters:
  name:
    type: "choice"
    values: ["Alice", "Bob", "Charlie"]
  age:
    type: "randomInt"
    min: 20
    max: 50
```
**Output**: Rendered template (e.g., `"Hello Alice, you are 35 years old"`)

##### `object` ✅
Generates JSON objects with specified properties.
```yaml
type: "object"
properties:
  name:
    type: "choice"
    values: ["John", "Jane", "Mike"]
  age:
    type: "randomInt"
    min: 18
    max: 65
  active:
    type: "choice"
    values: [true, false]
```
**Output**: JSON object (e.g., `{"name": "John", "age": 42, "active": true}`)

##### `array` ✅
Generates arrays with random length and elements.
```yaml
type: "array"
minLength: 1
maxLength: 5
elementGenerator:
  type: "randomInt"
  min: 1
  max: 100
```
**Output**: Array of values (e.g., `[42, 17, 89]`)

### Endpoint Configuration

Each endpoint defines how to make requests to a specific API path.

```yaml
endpoints:
  endpoint_name:
    path: "/api/path/{param}"    # URL path with parameter placeholders
    method: "GET"                # HTTP method
    headers:                     # Optional: custom headers
      Content-Type: "application/json"
      Authorization: "Bearer token"
    
    # Parameters for URL path (replace {param} placeholders)
    pathParameters:
      param:
        $ref: "generator_name"   # Reference to named generator
      # or inline definition:
      param:
        type: "randomInt"
        min: 1
        max: 100
    
    # URL query parameters (?key=value)
    queryParameters:
      limit:
        type: "randomInt"
        min: 10
        max: 50
      filter:
        $ref: "active_filter"    # Reference to named generator
      format: "json"             # Static string value
    
    # Request body (for POST, PUT, PATCH requests)
    bodyParameters:
      type: "object"
      properties:
        user:
          type: "object"
          properties:
            name:
              $ref: "user_name_generator"
            age:
              $ref: "user_age_generator"
```

### Endpoint Selection Strategies

Controls how endpoints are chosen during testing.

#### `roundRobin` ✅ **Implemented**
Cycles through endpoints in order, ensuring equal distribution.
```yaml
endpointSelection:
  strategy: "roundRobin"
```

#### `weighted` ✅ **Implemented**
Selects endpoints based on specified weights (probabilities).
```yaml
endpointSelection:
  strategy: "weighted"
  weights:
    get_user: 0.6      # 60% of requests
    create_user: 0.3   # 30% of requests
    delete_user: 0.1   # 10% of requests
```

#### `random` ✅ **Implemented**
Randomly selects endpoints with equal probability.
```yaml
endpointSelection:
  strategy: "random"
```

### Execution Modes

#### Fixed Mode ✅ **Implemented**
Maintains a constant request rate throughout the test duration.
```yaml
execution:
  mode: "fixed"
  durationSeconds: 300         # Run for 5 minutes
  requestsPerSecond: 100       # Constant 100 RPS
  requestTimeoutMs: 2000       # 2 second timeout per request
```

#### Ramp Mode 🚧 **Future Feature**
*Gradually increases or decreases request rate over time. This mode is planned for future implementation.*

### Complete Example

The repository includes several example configurations:

- ✅ **`config-examples/simple-example.yml`**: Basic configuration demonstrating GET and POST endpoints with parameter generation
- ✅ **`config.yaml`**: Default configuration file (copy of simple-example.yml)
- ✅ **`config-clean-example.yaml`**: Enhanced example showcasing the new `$ref` syntax and static strings

⚠️ **Note**: Some older configuration files in `config-examples/` may use outdated syntax or unsupported generator types. Use the files marked with ✅ above as reliable templates for your own configurations.

Each example demonstrates different aspects of the configuration system and can be used as templates for your own tests.

### Modes

#### fixed ✅ **Implemented**
Sends a fixed number of requests per second (RPS) to the defined endpoints. Endpoints are selected based on the configured strategy (round-robin, weighted, or random). This mode is fully implemented and recommended for most load testing scenarios.

## Output and Reporting

The tool provides comprehensive reporting including:
- Request count and success/failure rates
- Response time statistics (min, max, average)
- Error rate percentage
- Status code distribution
- Detailed error message summary with occurrence counts
- Execution duration and actual RPS achieved

Example output:
```
--- Benchmark Report ---
Test Mode: fixed
Duration: 60 seconds
Configured RPS: 50
Total Requests: 2998
Successful Requests: 2950
Failed Requests: 48
Error Rate: 1.60%
Min Response Time: 12.5ms
Max Response Time: 1.2s
Average Response Time: 85.3ms

Status Code Distribution:
  Status 200: 2950
  Status 500: 48

Error Message Summary:
  'Connection timeout': 35 times
  'Internal server error': 13 times
```

## TODO / Future Features
- [ ] **Implement ramp mode** (dynamic RPS adjustment over time)
- [ ] **Add unlimited/burst mode** (send requests as fast as possible)
- [ ] **Add latency percentile reporting** (p50, p90, p95, p99)
- [ ] **Support for additional authentication schemes** (OAuth, API keys)
- [ ] **CLI flags for overriding config values**
- [ ] **Real-time metrics dashboard/visualization**
- [ ] **Export results to various formats** (JSON, CSV, HTML reports)
- [ ] **Dockerfile for containerized runs**
- [ ] **CI/CD integration and automated tests**
- [ ] **Support for request dependencies and chaining**
- [ ] **Custom validation rules for response content**
- [ ] **Add sequence generator type** (incrementing numbers)
- [ ] **Add date/time generator types**

## Contributing
Contributions are welcome! Please open issues or submit pull requests for new features, bug fixes, or improvements.

## License
MIT License
