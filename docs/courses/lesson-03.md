# Lesson 3: Cloud Providers and Storage Routing

## Objectives

- Manage multi-cloud credentials with the `CloudProvider` interface
- Route asset paths to storage backends using the `Resolver`

## Concepts

### CloudProvider Interface

```go
type CloudProvider interface {
    Name() string
    Credentials() map[string]string
    HealthCheck(ctx context.Context) error
}
```

Three implementations are provided: `AWSProvider`, `GCPProvider`, and `AzureProvider`. Each validates required credentials on construction and exposes them as a key-value map.

### The Resolver

`resolver.Resolver` maps logical asset paths to storage backends using prefix-based rules. It implements the Strategy pattern for backend selection.

```go
type Backend interface {
    Name() string
    Read(ctx context.Context, path string) (io.ReadCloser, error)
    Write(ctx context.Context, path string, data io.Reader) error
    Exists(ctx context.Context, path string) (bool, error)
    Delete(ctx context.Context, path string) error
}
```

## Code Walkthrough

### Cloud providers

```go
aws, _ := provider.NewAWSProvider("AKID", "secret", "us-east-1", nil)
gcp, _ := provider.NewGCPProvider("my-project", "us-central1", nil)
azure, _ := provider.NewAzureProvider("sub-id", "tenant-id", nil)

// Optional chaining
aws.WithSessionToken("session-token")
gcp.WithServiceAccount("sa@project.iam.gserviceaccount.com")
azure.WithClientCredentials("client-id", "client-secret")

// Health check
for _, p := range []provider.CloudProvider{aws, gcp, azure} {
    if err := p.HealthCheck(ctx); err != nil {
        log.Printf("%s: %v", p.Name(), err)
    }
}
```

### Setting up the resolver

```go
r := resolver.New()
r.RegisterBackend(localBackend)  // implements resolver.Backend
r.RegisterBackend(s3Backend)

r.AddRule("thumbnails/", "local")
r.AddRule("originals/", "s3")
r.SetFallback("local")
```

### Using the resolver

```go
// Reads route through rules
reader, err := r.Read(ctx, "thumbnails/photo-sm.jpg") // -> local backend
reader, err = r.Read(ctx, "originals/photo.jpg")       // -> s3 backend

// Writes follow the same routing
r.Write(ctx, "thumbnails/new.jpg", imageReader)

// Resolve manually if needed
backend, err := r.Resolve("originals/data.bin")
fmt.Println(backend.Name()) // "s3"
```

The resolver is thread-safe and can be reconfigured at runtime by adding new rules or backends.

## Practice Exercise

1. Create an `AWSProvider` and verify `Credentials()` returns the correct keys. Test validation by omitting the access key and verifying the constructor returns an error.
2. Set up a `Resolver` with two backends (local for `thumbnails/`, mock S3 for `originals/`) and a local fallback. Write to both prefixes and verify each routes to the correct backend.
3. Create all three cloud providers (AWS, GCP, Azure) and call `HealthCheck` on each. Since no actual cloud connection exists, verify the appropriate error messages are returned.
