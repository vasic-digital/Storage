# FAQ

## How does the local client store metadata?

When you pass `WithContentType` or `WithMetadata` to `PutObject`, the local client writes a `.meta` JSON sidecar file alongside the object. For example, uploading `photos/cat.jpg` creates both `photos/cat.jpg` and `photos/cat.jpg.meta`. The sidecar is read back automatically by `StatObject` and `ListObjects`.

## Can I use this with AWS S3 directly?

Yes. The `s3` package uses the MinIO Go SDK, which is fully compatible with AWS S3. Point the `Endpoint` to `s3.amazonaws.com`, set `UseSSL: true`, and provide your AWS credentials.

## What is the difference between ObjectStore and BucketManager?

They are separate interfaces following the Interface Segregation Principle. `ObjectStore` handles object CRUD (put, get, delete, list, stat, copy, health check). `BucketManager` handles bucket operations (create, delete, list, exists). Both `s3.Client` and `local.Client` implement both interfaces.

## How does the resolver decide which backend to use?

The resolver iterates rules in order and returns the first backend whose prefix matches the path. If no rule matches and a fallback is configured, the fallback backend is used. If neither matches, an error is returned.

## Does the S3 client support multipart uploads?

Yes. The MinIO SDK handles multipart uploads automatically based on `PartSize` (default 16 MB). Objects larger than `PartSize` are split into parts and uploaded with up to `ConcurrentUploads` (default 4) parallel streams.
