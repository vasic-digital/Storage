# Examples

## 1. S3 Bucket with Versioning and Retention

Create a bucket with versioning enabled and a 90-day retention policy.

```go
package main

import (
    "context"

    "digital.vasic.storage/pkg/object"
    "digital.vasic.storage/pkg/s3"
)

func main() {
    cfg := s3.DefaultConfig()
    client, _ := s3.NewClient(cfg, nil)
    ctx := context.Background()
    client.Connect(ctx)
    defer client.Close()

    client.CreateBucket(ctx, object.BucketConfig{
        Name:          "documents",
        Versioning:    true,
        RetentionDays: 90,
    })

    // Use the builder API for bucket config
    bucketCfg := s3.DefaultBucketConfig("audit-logs").
        WithVersioning().
        WithRetention(365).
        WithObjectLocking()
}
```

## 2. Cloud Provider Credentials

Manage credentials for multi-cloud environments.

```go
package main

import (
    "context"
    "fmt"

    "digital.vasic.storage/pkg/provider"
)

func main() {
    ctx := context.Background()

    aws, _ := provider.NewAWSProvider(
        "AKIAIOSFODNN7EXAMPLE",
        "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
        "us-east-1",
        nil, // default config
    )
    aws.WithSessionToken("FwoGZX...")

    gcp, _ := provider.NewGCPProvider("my-project", "us-central1", nil)
    gcp.WithServiceAccount("sa@project.iam.gserviceaccount.com")

    azure, _ := provider.NewAzureProvider("sub-id", "tenant-id", nil)
    azure.WithClientCredentials("client-id", "client-secret")

    providers := []provider.CloudProvider{aws, gcp, azure}
    for _, p := range providers {
        err := p.HealthCheck(ctx)
        fmt.Printf("%-6s healthy=%v creds=%v\n",
            p.Name(), err == nil, len(p.Credentials()))
    }
}
```

## 3. Multi-Backend Storage Resolver

Route reads and writes to different storage backends by path prefix.

```go
package main

import (
    "context"
    "fmt"
    "strings"

    "digital.vasic.storage/pkg/resolver"
)

// memBackend is a trivial in-memory backend for demonstration.
type memBackend struct {
    name string
    data map[string]string
}

func (b *memBackend) Name() string { return b.name }
func (b *memBackend) Read(_ context.Context, path string) (io.ReadCloser, error) {
    return io.NopCloser(strings.NewReader(b.data[path])), nil
}
func (b *memBackend) Write(_ context.Context, path string, data io.Reader) error {
    buf, _ := io.ReadAll(data)
    b.data[path] = string(buf)
    return nil
}
func (b *memBackend) Exists(_ context.Context, path string) (bool, error) {
    _, ok := b.data[path]
    return ok, nil
}
func (b *memBackend) Delete(_ context.Context, path string) error {
    delete(b.data, path)
    return nil
}

func main() {
    hot := &memBackend{name: "hot", data: map[string]string{}}
    cold := &memBackend{name: "cold", data: map[string]string{}}

    r := resolver.New()
    r.RegisterBackend(hot)
    r.RegisterBackend(cold)
    r.AddRule("cache/", "hot")
    r.AddRule("archive/", "cold")
    r.SetFallback("hot")

    ctx := context.Background()
    r.Write(ctx, "cache/session.json", strings.NewReader(`{"user":"alice"}`))
    r.Write(ctx, "archive/2025.tar.gz", strings.NewReader("data"))

    exists, _ := r.Exists(ctx, "cache/session.json")
    fmt.Println("cache/session.json exists:", exists) // true
}
```
