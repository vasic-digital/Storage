# Storage — `digital.vasic.storage`

Generic, reusable Go module for object-storage operations. Provides
unified `ObjectStore` / `BucketManager` interfaces over three concrete
backends — S3-compatible (MinIO, AWS S3), local filesystem with
sidecar JSON metadata, and cloud-provider credential management.

**Module ID**: `digital.vasic.storage`
**Toolchain**: Go 1.25.0+
**Status**: production
**Distribution model**: standalone submodule under `vasic-digital/*`;
consumers reference it via `go.mod replace` or vendoring.

---

## Anti-bluff guarantees (CONST-035 / Article XI §11.9)

Every PASS this module reports carries positive runtime evidence:

1. **Real filesystem I/O on the local backend** — `pkg/local`
   exercises real `os.MkdirAll`, real `io.Copy`, real sidecar
   `.meta` JSON marshal + write. The unit and edge tests assert
   actual file content, not return-value shape.
2. **Real MinIO/S3 wire protocol on the S3 backend** — `pkg/s3`
   speaks real `minio-go/v7`; `cloudfront.go` performs real
   RSA-SHA1 URL signing (resolved round-38 from
   `ErrCloudFrontSigningNotWired` sentinel to a wired implementation).
3. **Defensive-use boundary on every external entry point** — no
   payload generator, no obfuscator, no bypass helper; reads /
   writes only with caller-supplied buckets, keys, and payloads.
4. **Paired-mutation Challenge** —
   `challenges/storage_describe_challenge.sh --mutate` plants a
   schema violation (sidecar `.meta` with empty `Key`) and asserts
   the invariant check surfaces it as exit `99`. A mutation run
   that exits `0` flags the Challenge itself as a bluff.
5. **5-locale bilingual UX evidence per CONST-046** — the runner
   emits `en` / `sr` / `ja` / `es` / `de` operator summary lines
   every invocation; locale drift is caught by `challenges/fixtures/locales.yaml`.

> **Verbatim 2026-05-19 operator mandate** (Article XI §11.9):
> *"all existing tests and Challenges do work in anti-bluff manner —
> they MUST confirm that all tested codebase really works as
> expected! We had been in position that all tests do execute with
> success and all Challenges as well, but in reality the most of the
> features does not work and can't be used! This MUST NOT be the
> case and execution of tests and Challenges MUST guarantee the
> quality, the completition and full usability by end users of the
> product!"*

---

## Packages

| Package         | Purpose                                                                | Real-infrastructure surface                                |
|-----------------|------------------------------------------------------------------------|------------------------------------------------------------|
| `pkg/object`    | Core interfaces (`ObjectStore`, `BucketManager`) + shared types         | Pure types; no I/O.                                        |
| `pkg/s3`        | S3-compatible client (MinIO, AWS S3) + presigned URLs + lifecycle rules | Real `minio-go/v7` over HTTP(S); real RSA-SHA1 CF signing. |
| `pkg/local`     | Local filesystem backend with sidecar `.meta` JSON metadata             | Real `os.*` syscalls; real `encoding/json` marshalling.    |
| `pkg/provider`  | Cloud provider credential management (AWS, GCP, Azure)                  | Real credential-chain resolution.                          |
| `pkg/recording` | Audio/video recording sidecar metadata (S3 upload wired round-37)       | Real S3 `PutObject` via `pkg/s3`.                          |
| `pkg/netstorage`| Network-storage shared helpers                                         | Stateless helpers; consumers add transport.                |
| `pkg/resolver`  | Path / URI resolution helpers                                          | Stateless; pure functions.                                 |
| `pkg/i18n`      | Operator-facing locale strings (CONST-046)                             | YAML-driven; round-118 + round-216 migration complete.     |

---

## Quick start

```go
import (
    "context"

    "digital.vasic.storage/pkg/local"
    "digital.vasic.storage/pkg/object"
)

ctx := context.Background()
cli, err := local.NewClient(&local.Config{RootDir: "/var/lib/myapp/storage"}, nil)
if err != nil { panic(err) }
if err := cli.Connect(ctx); err != nil { panic(err) }
defer cli.Close()

if err := cli.CreateBucket(ctx, object.BucketConfig{Name: "documents"}); err != nil {
    panic(err)
}
// PutObject + GetObject + ListObjects on `documents` — real files on disk.
```

For S3 / MinIO:

```go
import "digital.vasic.storage/pkg/s3"

cli, _ := s3.NewClient(s3.DefaultConfig(), nil)
_ = cli.Connect(ctx)
defer cli.Close()
// Real HTTP(S) traffic against the configured endpoint.
```

---

## Testing — anti-bluff invariants

Resource-capped per universal rule:

```bash
GOMAXPROCS=2 nice -n 19 ionice -c 3 \
  go test -count=1 -p 1 -race ./...
```

- **Unit** — `*_test.go` in every `pkg/*` — mocks permitted ONLY
  here per CONST-050(A). Edge tests probe partial-write failures,
  invalid sidecar JSON, missing buckets, oversize objects.
- **Integration** — `tests/integration/` — real MinIO container,
  real filesystem mounts. Skipped with `SKIP-OK:` markers when
  infrastructure unavailable per CONST-035 skip-bluff clause.
- **E2E** — `tests/e2e/` — full upload/download/list/delete
  lifecycle against real backends.
- **Security** — `tests/security/` — credential-leak scans,
  presigned-URL expiry checks, lifecycle-rule honour-system tests.
- **Chaos / Stress / DDoS / Scaling / Smoke / Benchmark / Recording / Full-auto** —
  all under `tests/<type>/` per CONST-050(B).
- **Challenges** — `challenges/scripts/*.sh` — protocol-layer probes
  with the four-layer test floor (compile + unit + functionality + chaos).
- **Round-283 deep-doc** — `challenges/storage_describe_challenge.sh`
  default mode exercises the runner against the real public API;
  `--mutate` mode plants a sidecar-schema violation and asserts the
  invariant check trips it (exit `99` = mutation correctly surfaced).

---

## Test-coverage ledger

Symbol → test → Challenge mapping lives in
[`docs/test-coverage.md`](./docs/test-coverage.md). Every exported
symbol MUST appear there; adding an exported symbol without
updating the ledger is a CONST-048 violation.

---

## Defensive-use posture

This module is read/write infrastructure plumbing. It does NOT:

- generate / obfuscate / encrypt payloads (that's `digital.vasic.encryption`);
- proxy / route arbitrary traffic (that's `digital.vasic.networking`);
- enumerate buckets across organisations (the caller's auth scope governs visibility);
- evade lifecycle / quota / retention policies — it ENFORCES them via `SetLifecycleRule`.

The presigned-URL helper produces short-TTL URLs (caller-specified)
backed by the configured AWS/MinIO credentials — exactly the same
surface `aws s3 presign` exposes.

---

## Governance & cascade

- `CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md` cascade every active
  anchor (CONST-033 … CONST-061) from the constitution submodule.
- `.gitignore` enforces CONST-053 (no versioned build artefacts;
  no `.env*` except `.env.example`).
- `Upstreams/` directory feeds `install_upstreams` per CONST-056 —
  four remotes (`origin`, `github`, `gitlab`, `upstream`).

## License

Proprietary — `vasic-digital`. See submodule analysis for downstream
consumer list.
