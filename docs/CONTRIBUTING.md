# Contributing to Storage Module

## Getting Started

### Prerequisites

- Go 1.24 or later
- MinIO (for integration tests): `docker run -p 9000:9000 minio/minio server /data`
- `golangci-lint` (optional, for linting)

### Clone and Build

```bash
git clone <repository-url>
cd Storage
go build ./...
```

### Run Tests

```bash
# Unit tests
go test ./... -count=1 -race

# Short mode (skip integration)
go test ./... -short

# Integration tests (requires MinIO)
go test -tags=integration ./...

# Specific package
go test -v ./pkg/s3/...

# Specific test
go test -v -run TestClient_BucketOperations ./pkg/local/
```

## Development Standards

### Code Style

- Standard Go conventions per [Effective Go](https://go.dev/doc/effective_go)
- Format with `gofmt` or `goimports`
- Maximum line length: 100 characters
- Imports grouped and blank-line separated: stdlib, third-party, internal

### Naming Conventions

| Kind | Convention | Example |
|------|-----------|---------|
| Private fields/functions | camelCase | `rootDir`, `writeSidecar` |
| Exported types/functions | PascalCase | `ObjectStore`, `NewClient` |
| Constants | UPPER_SNAKE_CASE | (none currently, but follow if added) |
| Acronyms | All caps | `URL`, `ID`, `SSL` |
| Receivers | 1-2 letters | `c` for Client, `p` for Provider |

### Error Handling

- Always check errors. Never discard with `_` unless in defer cleanup.
- Wrap errors with context: `fmt.Errorf("failed to create bucket: %w", err)`
- Use consistent prefixes: `"failed to ..."`, `"not connected to ..."`
- Preserve error chain with `%w` for `errors.Is`/`errors.As` compatibility.

### Interface Design

- Keep interfaces small and focused (Interface Segregation Principle).
- Accept interfaces, return structs.
- Add compile-time compliance checks: `var _ Interface = (*Struct)(nil)`

### Concurrency

- Protect shared state with `sync.RWMutex`.
- Use write locks only for state mutations (Connect, Close).
- Use read locks for all other operations.
- Always pass `context.Context` as the first parameter.

## Adding a New Storage Backend

1. Create a new package under `pkg/` (e.g., `pkg/gcs/`).
2. Define a `Client` struct and a `Config` struct.
3. Implement `object.ObjectStore` and `object.BucketManager`.
4. Add compile-time interface checks.
5. Add a `NewClient(config, logger)` constructor with validation.
6. Add `Connect`, `Close`, `IsConnected`, and `HealthCheck` methods.
7. Protect connection state with `sync.RWMutex`.
8. Add comprehensive tests (see Testing section below).

## Adding a New Cloud Provider

1. Define a struct in `pkg/provider/provider.go` with credential fields.
2. Implement the `CloudProvider` interface (Name, Credentials, HealthCheck).
3. Add a `New*Provider` constructor with required-field validation.
4. Add builder methods (`With*`) returning `*T` for chaining.
5. Add compile-time check: `var _ CloudProvider = (*NewProvider)(nil)`
6. Add table-driven tests in `pkg/provider/provider_test.go`.

## Adding a New PutOption

1. Add a new field to `putOptions` in `pkg/object/object.go`.
2. Create a `With*` function returning `PutOption`.
3. Update `ResolvePutOptions` if special handling is needed.
4. Update `s3.Client.PutObject` to honor the new option.
5. Update `local.Client.PutObject` to honor the new option.
6. Add tests for the new option in `pkg/object/object_test.go`.
7. Add tests for each backend using the new option.

## Testing Requirements

### Mandatory Tests

Every exported function and method must have tests covering:

- **Happy path**: Normal successful operation.
- **Validation errors**: Invalid inputs, missing required fields.
- **Not-connected guard**: Operations before `Connect` must return `"not connected"` errors.
- **Edge cases**: Nil inputs, empty strings, zero values.

### Test Structure

Use table-driven tests with `testify`:

```go
func TestClient_Method_Scenario(t *testing.T) {
    tests := []struct {
        name        string
        input       InputType
        expectError bool
        errorMsg    string
    }{
        {
            name:        "valid input",
            input:       validInput,
            expectError: false,
        },
        {
            name:        "invalid input",
            input:       invalidInput,
            expectError: true,
            errorMsg:    "expected error substring",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := client.Method(tt.input)
            if tt.expectError {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorMsg)
            } else {
                require.NoError(t, err)
                assert.NotNil(t, result)
            }
        })
    }
}
```

### Test Helpers

Use `t.TempDir()` for temporary directories and `t.Helper()` in helper functions:

```go
func newTestClient(t *testing.T) *Client {
    t.Helper()
    dir := t.TempDir()
    client, err := NewClient(&Config{RootDir: dir}, nil)
    require.NoError(t, err)
    err = client.Connect(context.Background())
    require.NoError(t, err)
    return client
}
```

### Concurrency Tests

All client types must have concurrency tests verifying thread safety:

```go
func TestClient_Concurrency(t *testing.T) {
    client := newTestClient(t)
    done := make(chan bool, 20)
    for i := 0; i < 10; i++ {
        go func() {
            _ = client.IsConnected()
            done <- true
        }()
    }
    for i := 0; i < 10; i++ {
        go func() {
            _ = client.Close()
            done <- true
        }()
    }
    for i := 0; i < 20; i++ {
        <-done
    }
}
```

## Commit Conventions

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>
```

**Types**: `feat`, `fix`, `test`, `refactor`, `docs`, `chore`

**Scopes**: `object`, `s3`, `local`, `provider`, `storage`

**Examples**:

```
feat(s3): add presigned URL generation
fix(local): handle missing sidecar on StatObject
test(provider): add Azure service principal tests
refactor(object): extract option resolution logic
docs(storage): update architecture diagram
chore(deps): bump minio-go to v7.0.83
```

## Pull Request Process

1. Create a feature branch: `feat/description` or `fix/description`.
2. Ensure all tests pass: `go test ./... -count=1 -race`.
3. Run `gofmt` on all changed files.
4. Verify that `go vet ./...` produces no warnings.
5. Write a clear PR description explaining the change and its motivation.
6. Link to any related issues.

## Dependencies

Current dependencies:

| Dependency | Purpose |
|-----------|---------|
| `github.com/minio/minio-go/v7` | S3-compatible client SDK |
| `github.com/sirupsen/logrus` | Structured logging |
| `github.com/stretchr/testify` | Test assertions (test only) |

When adding new dependencies:
- Prefer the standard library where possible.
- Evaluate the dependency's maintenance status and license.
- Run `go mod tidy` after changes.
- Commit `go.mod` and `go.sum` together.
