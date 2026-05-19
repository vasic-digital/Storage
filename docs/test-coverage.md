# test-coverage.md — `digital.vasic.storage`

Round-283 symbol → test / Challenge ledger. Every exported symbol of
`digital.vasic.storage` MUST appear here with the test(s) and
Challenge(s) that exercise it AND the anti-bluff dimension each
proves. Adding an exported symbol without updating this ledger is
a CONST-048 violation. Per Article XI §11.9, every PASS row MUST
carry positive runtime evidence — the "Evidence" column documents
what to capture during a release-gate sweep.

## Exported symbols — interface layer (`pkg/object`)

| Symbol               | Kind      | Unit test(s)                              | Challenge(s)                             | Anti-bluff dimension                                                       | Evidence (runtime)                                                         |
|----------------------|-----------|-------------------------------------------|------------------------------------------|----------------------------------------------------------------------------|----------------------------------------------------------------------------|
| `ObjectStore`        | iface     | `pkg/object/object_test.go`               | `runner -describe`                       | Interface satisfied by both `local.Client` and `s3.Client` at compile time | `go vet ./...` clean; runner asserts both impls return non-nil interface.  |
| `BucketManager`      | iface     | `pkg/object/object_test.go`               | `runner -describe`                       | Same as above; bucket-management surface is consistent across backends.    | `go vet ./...` clean; runner enumerates implementing types.                |
| `ObjectRef`          | struct    | `pkg/object/object_test.go`               | `runner -describe`                       | Bucket+Key composition round-trips.                                        | Runner prints struct shape line.                                           |
| `ObjectInfo`         | struct    | `pkg/object/object_test.go`               | `runner -describe`                       | Metadata fields preserved across backend boundary.                         | Runner asserts non-zero Size + LastModified after PutObject.               |
| `BucketInfo`         | struct    | `pkg/object/object_test.go`               | `runner -describe`                       | Bucket listing returns same shape from local + S3.                         | Runner enumerates buckets, asserts shape.                                  |
| `BucketConfig`       | struct    | `pkg/object/object_test.go`               | `runner -describe`                       | Bucket-creation parameters honoured.                                       | Runner creates + deletes a temp bucket.                                    |
| `PutOption`          | type      | `pkg/object/object_test.go`               | `runner -describe`                       | Functional-options pattern; nil-safe.                                      | Unit test exercises `WithContentType` + `WithMetadata`.                    |
| `WithContentType`    | func      | `pkg/object/object_test.go`               | `runner -describe`                       | ContentType propagates to ObjectInfo on Stat.                              | Unit test asserts round-trip.                                              |
| `WithMetadata`       | func      | `pkg/object/object_test.go`               | `runner -describe`                       | User metadata propagates to ObjectInfo on Stat.                            | Unit test asserts round-trip.                                              |
| `ResolvePutOptions`  | func      | `pkg/object/object_test.go`               | `runner -describe`                       | Variadic option resolution stable + order-independent.                     | Unit test asserts deterministic merge.                                     |

## Exported symbols — local backend (`pkg/local`)

| Symbol                 | Kind   | Unit test(s)                                                  | Challenge(s)                                                                            | Anti-bluff dimension                                                                | Evidence (runtime)                                                |
|------------------------|--------|---------------------------------------------------------------|-----------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------|-------------------------------------------------------------------|
| `Config`               | struct | `pkg/local/client_test.go`                                    | `runner -all`, `storage_describe_challenge.sh`                                          | Config rejects empty `RootDir`; default logger initialised.                         | Unit test asserts `nil`-config → error.                           |
| `Client`               | struct | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | `Connect` creates root dir on real filesystem; `IsConnected` honest.                | Runner mkdtemp + connect + asserts directory present.             |
| `NewClient`            | func   | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | Constructor returns wired client; logger fallback applied.                          | Runner enumerates struct fields via reflection.                   |
| `(*Client).Connect`    | method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | Real `os.MkdirAll` call; idempotent on repeat invocation.                            | Runner asserts root exists on disk after Connect.                 |
| `(*Client).Close`      | method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | `connected` flag flipped; no resources leaked.                                       | Runner asserts `IsConnected() == false` after Close.              |
| `(*Client).IsConnected`| method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | Truthful state reporting; mu-protected.                                              | Runner asserts before/after Connect transitions.                  |
| `(*Client).HealthCheck`| method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | `os.Stat` on real root; non-existence → error.                                       | Runner exercises healthy + missing-root paths.                    |
| `(*Client).CreateBucket`| method | `pkg/local/client_test.go`, `pkg/local/client_edge_test.go`  | `runner -all`                                                                            | Real `os.MkdirAll` for bucket dir; rejects empty name.                               | Runner creates `tmp-bucket`, asserts on-disk dir present.         |
| `(*Client).DeleteBucket`| method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | Real `os.RemoveAll`; rejects non-empty bucket without force flag (caller-policy).    | Runner deletes `tmp-bucket`, asserts gone.                        |
| `(*Client).ListBuckets`| method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | Real `os.ReadDir`; includes only directories (sidecars filtered).                    | Runner enumerates buckets, asserts `tmp-bucket` present.          |
| `(*Client).BucketExists`| method | `pkg/local/client_test.go`                                   | `runner -all`                                                                            | Truthful presence reporting.                                                         | Runner asserts true/false transitions.                            |
| `(*Client).PutObject`  | method | `pkg/local/client_test.go`, `pkg/local/client_edge_test.go`  | `runner -all`, `storage_describe_challenge.sh --mutate`                                  | Real `io.Copy` to disk + real sidecar `.meta` JSON write; partial-write rolled back.| Runner upload payload, asserts file + sidecar content on disk.    |
| `(*Client).GetObject`  | method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | Real `os.Open`; `ReadCloser` closes underlying file.                                 | Runner round-trip read; asserts byte-equality.                    |
| `(*Client).DeleteObject`| method | `pkg/local/client_test.go`                                   | `runner -all`                                                                            | Real `os.Remove` on both payload + sidecar; missing-object → error.                  | Runner deletes, asserts both gone.                                |
| `(*Client).ListObjects`| method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | Real walk of bucket dir; sidecar files filtered from listing.                        | Runner enumerates after upload.                                   |
| `(*Client).StatObject` | method | `pkg/local/client_test.go`, `pkg/local/client_edge_test.go`  | `runner -all`, `storage_describe_challenge.sh --mutate`                                  | Reads sidecar JSON; corrupted sidecar → error (not silent zero-info).                | Default Challenge asserts content-type round-trip; mutate plants empty Key → exit 99. |
| `(*Client).CopyObject` | method | `pkg/local/client_test.go`                                    | `runner -all`                                                                            | Real file copy + sidecar copy; preserves metadata.                                   | Runner copies + stats destination.                                |

## Exported symbols — S3 backend (`pkg/s3`)

S3 backend symbols are exercised by `pkg/s3/client_test.go` (unit
with mock `MinioClientFactory`), `pkg/s3/cloudfront_test.go` (real
RSA-SHA1 signing — round-38 fix), and `tests/integration/`
(real MinIO container). Detailed per-method ledger lives in
[`docs/API_REFERENCE.md`](API_REFERENCE.md) and the inline godoc.

| Symbol                      | Unit test(s)                                | Challenge(s)             | Anti-bluff dimension                                                |
|-----------------------------|---------------------------------------------|--------------------------|---------------------------------------------------------------------|
| `Client`, `NewClient`       | `pkg/s3/client_test.go`                     | `storage_unit_challenge` | Constructor + factory injection for test isolation.                 |
| `(*Client).Connect/Close`   | `pkg/s3/client_test.go`                     | `storage_unit_challenge` | Real or factory-supplied `minio-go/v7` lifecycle.                   |
| `(*Client).PutObject` …     | `pkg/s3/client_test.go`                     | `storage_unit_challenge` | Real bucket+key flow; sentinel `ErrS3UploadNotWired` resolved r-37. |
| `(*Client).GetPresignedURL` | `pkg/s3/client_test.go`                     | `storage_unit_challenge` | Real presigned-URL TTL honour.                                      |
| `SignCloudFrontURL`         | `pkg/s3/cloudfront_test.go`                 | `storage_unit_challenge` | Real RSA-SHA1 signing — sentinel resolved round-38.                 |
| `SetLifecycleRule` …        | `pkg/s3/client_test.go`                     | `storage_unit_challenge` | Real lifecycle rule round-trip.                                     |
| `Config`, `DefaultConfig`   | `pkg/s3/config_test.go`                     | `storage_unit_challenge` | Validation rejects empty endpoint/region.                           |

## Anti-bluff dimensions covered

| Dimension                                                          | Where proved                                                                                   |
|--------------------------------------------------------------------|------------------------------------------------------------------------------------------------|
| Interface satisfaction (compile + runtime)                         | `go vet ./...` + runner's interface-satisfaction check.                                        |
| Real filesystem syscalls on local backend                          | `pkg/local/client_test.go` + runner mkdtemp+connect+assert.                                    |
| Real `minio-go/v7` wire protocol                                   | `tests/integration/` against real MinIO container.                                             |
| Real RSA-SHA1 CloudFront URL signing                               | `pkg/s3/cloudfront_test.go` (round-38 fix).                                                    |
| Sidecar JSON metadata round-trip integrity                         | `pkg/local/client_edge_test.go` invalid-JSON + truncated-file cases.                           |
| Paired-mutation surfaces schema violations                         | `challenges/storage_describe_challenge.sh --mutate` exit 99.                                   |
| 5-locale operator UX evidence (CONST-046)                          | `challenges/runner/main.go` + `challenges/fixtures/locales.yaml`.                              |
| `.env` / build-artefact discipline (CONST-053)                     | `.gitignore` audit + `git ls-files --error-unmatch`.                                           |
| Defensive-use boundary (no payload generators, no obfuscators)     | Challenge greps for `Generate(Payload|Attack|Obfuscat)` in non-test sources; must be empty.   |

## Maintenance

When you add an exported symbol:

1. Add a row to the matching table above with a real test + Challenge entry.
2. Update `challenges/runner/main.go` if the symbol participates in the
   runtime invariant assertions.
3. Re-run `bash challenges/storage_describe_challenge.sh` (default
   AND `--mutate`) and capture the output.
4. Bump the round-N tag in commit message + this header.
