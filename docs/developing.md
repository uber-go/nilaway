# Developing

NilAway is a static analysis tool that detects potential nil panics in Go code. It uses
sophisticated analysis techniques to track nil flows within and across packages, reporting errors
with nilness flows for easier debugging.

## Building

```bash
make build                    # Build the nilaway binary to <project root>/bin/
```

## Testing

```bash
make test                     # Run unit tests for all modules
make cover                    # Run tests with coverage reports
make integration-test         # Run integration tests (using real drivers)
```

The following golden tests are available, which run NilAway on a base (usually `main`) and test (usually `HEAD`) branches
on stdlib and compare the differences of NilAway violations. This is mostly run in CI to catch unexpected breakages.

```bash
# The arguments are passed as an environment variable `ARGS`.
# Use `make golden-test ARGS="-h" to see the available arguments.
make golden-test ARGS="-base-branch main -test-branch HEAD -result-file /tmp/result.txt"
```

## Linting

```bash
make lint                    # Run all linting (format check, mod tidy, golangci-lint, nilaway self-check)
make lint-fix                # Run all linting with autofix (and auto-formats) applied
```

The following subcommands are available if you need to run individual linting components. Pass in `FIX=true` environment
variable to apply auto-fixes _if available_. 

In most cases you only need to ever run `make lint` or `make lint-fix` instead of these.

```bash
make format-lint             # Check if Go files are correctly formatted
make tidy-lint               # Check go.mod tidiness
make golangci-lint           # Run golangci-lint only
make nilaway-lint            # Run nilaway on itself
```

### Upgrading Dependencies

Run `make upgrade-deps` to upgrade all dependencies and tools to their latest versions for all modules within NilAway.

Optionally, set `GO_VERSION=<version>` environment variable to upgrade to a specific Go version (e.g., `1.21`).

### Running NilAway

```bash
# Build nilaway in current codebase.
make build

# Standalone usage
bin/nilaway -include-pkgs="<YOUR_PKG_PREFIX>" ./...

# With JSON output (disable pretty-print)
bin/nilaway -json -pretty-print=false -include-pkgs="<YOUR_PKG_PREFIX>" ./...

# Using custom golangci-lint build
golangci-lint custom         # Build custom binary with NilAway plugin
./custom-gcl run ./...       # Run custom golangci-lint with NilAway
```

## Architecture

For best performance and easier maintenance, NilAway consists of multiple levels of sub-analyzers that are all
`analysis.Analyzer`s, and they are connected by specifying dependencies (via `Requires` field) between them. Currently, the organization is as follows:

- **nilaway.Analyzer** (nilaway.go) - Top-level analyzer that reports errors
  - **accumulation.Analyzer** - Collects triggers, runs inference, returns errors
    - **annotation.Analyzer** - Reads annotations from structs/interfaces/functions
    - **function.Analyzer** - Analyzes functions and creates triggers
      - **anonymousfunc.Analyzer** - Handles function literals
      - **structfield.Analyzer** - Handles struct field accesses
    - **affiliation.Analyzer** - Creates interface-struct affiliation triggers
    - **global.Analyzer** - Creates global variable triggers

All the analyzers depend on `config.Analyzer` (`config/config.go`) to retrieve configurations.

The decoupling of error generation and error reporting logic makes it possible to apply custom error reporting to fit other needs. For example, it is possible to create another top-level analyzer `nilaway-log` that depends on the accumulation analyzer, which simply retrieves the NilAway errors and logs them to a local file or database for later auditing.

### Key Components

- **Triggers**: Flow conditions that may cause nil panics
- **Annotations**: Metadata about nilability of types and functions
- **Inference Engine**: Matches triggers with annotations to detect nil flows
- **Facts Mechanism**: Caches analysis results across packages for performance

### Important Files

- `nilaway.go` - Main analyzer entry point
- `cmd/nilaway/main.go` - Standalone checker with additional flags
- `cmd/gclplugin/gclplugin.go` - golangci-lint plugin integration
- `accumulation/analyzer.go` - Core inference coordination
- `config/config.go` - Configuration management

## Key Configuration Flags

- `-include-pkgs`: Comma-separated package prefixes to analyze (recommended)
- `-exclude-pkgs`: Package prefixes to exclude from analysis
- `-pretty-print`: Enable/disable pretty error messages (default: true)
- `-group-error-messages`: Group similar error messages
- `-experimental-struct-init-enable`: Enable experimental struct initialization
- `-experimental-anonymous-func-enable`: Enable experimental anonymous function support

## Performance Notes

- Uses `go/analysis` Facts mechanism for cross-package caching
- For large projects, use modular drivers (bazel/nogo or golangci-lint) over standalone checker
- Recommend using `-include-pkgs` to focus analysis on first-party code only

## Module Structure

- Root module: Core NilAway implementation
- `tools/` module: Development tools (golden-test, integration-test)
- `testdata/` directory: Comprehensive test cases organized by feature

## More docs
Refer to `docs/` directory for more documentations on other aspects of NilAway.