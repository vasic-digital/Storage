# AGENTS.md - Storage Module Multi-Agent Coordination Guide

## Overview

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working on the `digital.vasic.storage` module. It defines boundaries, responsibilities, and coordination rules to prevent conflicts when multiple agents operate concurrently.

## Module Identity

- **Module path**: `digital.vasic.storage`
- **Go version**: 1.24+
- **Repository root**: `Storage/`
- **Parent project**: HelixAgent (submodule)

## Package Ownership Map

| Package | Path | Responsibility |
|---------|------|---------------|
| `object` | `pkg/object/` | Core interfaces, shared types, functional options |
| `s3` | `pkg/s3/` | S3-compatible storage client (MinIO, AWS S3) |
| `local` | `pkg/local/` | Local filesystem storage backend |
| `provider` | `pkg/provider/` | Cloud provider credential management |

## Agent Roles

### Interface Agent

- **Scope**: `pkg/object/`
- **Responsibilities**: Maintain `ObjectStore` and `BucketManager` interfaces, shared types (`ObjectInfo`, `BucketInfo`, `BucketConfig`, `ObjectRef`, `Config`), and functional options (`PutOption`, `WithContentType`, `WithMetadata`).
- **Rules**:
  - Changes to interfaces require coordinated updates across all implementing packages (`pkg/s3/`, `pkg/local/`).
  - Never break backward compatibility without a major version bump.
  - All new option types must follow the functional options pattern (`PutOption func(*putOptions)`).

### S3 Backend Agent

- **Scope**: `pkg/s3/`
- **Responsibilities**: S3-compatible client implementation, config validation, bucket configuration builders, lifecycle rule management, presigned URL generation.
- **Rules**:
  - Must implement both `object.ObjectStore` and `object.BucketManager`.
  - Compile-time interface checks must be present: `var _ object.ObjectStore = (*Client)(nil)`.
  - All minio-go interactions must be behind the `sync.RWMutex`.
  - Connection state must be checked before every operation.

### Local Backend Agent

- **Scope**: `pkg/local/`
- **Responsibilities**: Filesystem-based storage, sidecar `.meta` JSON metadata files, directory-as-bucket mapping.
- **Rules**:
  - Must implement both `object.ObjectStore` and `object.BucketManager`.
  - Sidecar metadata files use the `.meta` suffix convention.
  - File paths must use `filepath.Join` and normalize to forward slashes for keys.
  - All operations require the `connected` guard.

### Provider Agent

- **Scope**: `pkg/provider/`
- **Responsibilities**: Cloud provider credential abstractions for AWS, GCP, and Azure.
- **Rules**:
  - Must implement the `CloudProvider` interface.
  - Credential fields must never be logged at INFO level or above.
  - Constructor functions must validate all required fields.
  - Builder methods (`With*`) must return the receiver for chaining.

## Coordination Rules

### Cross-Package Changes

1. **Interface changes** in `pkg/object/` trigger mandatory updates in:
   - `pkg/s3/client.go` (S3 implementation)
   - `pkg/local/client.go` (local implementation)
   - All corresponding `*_test.go` files

2. **New PutOption additions** in `pkg/object/` require:
   - Update `putOptions` struct
   - Add `With*` constructor function
   - Update `ResolvePutOptions` if needed
   - Update both S3 and local `PutObject` implementations to honor the new option

3. **New provider additions** in `pkg/provider/` require:
   - Implement `CloudProvider` interface
   - Add compile-time check: `var _ CloudProvider = (*NewProvider)(nil)`
   - Add constructor with validation
   - Add table-driven tests

### File Locking Protocol

When multiple agents work simultaneously:

- **Never** modify the same file concurrently.
- If Agent A is modifying `pkg/object/object.go`, Agent B must wait before touching `pkg/s3/client.go` (dependency).
- Test files can be modified independently as long as no new interface methods are being added.
- `go.mod` and `go.sum` changes must be serialized.

### Testing Requirements

- Every exported function and method must have a corresponding test.
- Tests must be table-driven using `testify/assert` and `testify/require`.
- Test naming convention: `Test<Struct>_<Method>_<Scenario>`.
- Concurrency safety tests are required for all client types.
- Not-connected guard tests are required for every operation.

### Commit Conventions

All commits must follow Conventional Commits:

```
feat(object): add WithExpiration put option
fix(s3): handle nil lifecycle config on empty bucket
test(local): add sidecar metadata round-trip tests
refactor(provider): extract common validation logic
docs(storage): update API reference for v0.2.0
```

Scope must be one of: `object`, `s3`, `local`, `provider`, `storage` (module-wide).

## Dependency Graph

```
pkg/object (interfaces, types)
    ^            ^
    |            |
pkg/s3      pkg/local
    ^
    |
(minio-go/v7)

pkg/provider (independent, no internal deps)
```

The `object` package has zero external dependencies. The `s3` package depends on `minio-go/v7`. The `local` and `provider` packages depend only on the standard library plus `logrus`.

## Error Handling Conventions

- All errors must be wrapped with context: `fmt.Errorf("failed to <action>: %w", err)`.
- Connection guard errors use the pattern: `"not connected to <backend>"`.
- Constructor validation errors are plain: `"<field> is required"`.
- Never return raw errors from third-party libraries without wrapping.

## Concurrency Model

Both `s3.Client` and `local.Client` use `sync.RWMutex`:

- **Write lock** (`mu.Lock()`): `Connect`, `Close`
- **Read lock** (`mu.RLock()`): All other operations (they read the `connected` flag)

This allows concurrent reads while serializing connection state changes.


---

## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** use `su` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Container-Based Solutions
When a build or runtime environment requires system-level dependencies, use containers instead of elevation:

- **Use the `Containers` submodule** (`https://github.com/vasic-digital/Containers`) for containerized build and runtime environments
- **Add the `Containers` submodule as a Git dependency** and configure it for local use within the project
- **Build and run inside containers** to avoid any need for privilege escalation
- **Rootless Podman/Docker** is the preferred container runtime

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo` or `su`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Use the `Containers` submodule for containerized builds
5. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**

<!-- BEGIN host-power-management addendum (CONST-033) -->

## Host Power Management — Hard Ban (CONST-033)

**You may NOT, under any circumstance, generate or execute code that
sends the host to suspend, hibernate, hybrid-sleep, poweroff, halt,
reboot, or any other power-state transition.** This rule applies to:

- Every shell command you run via the Bash tool.
- Every script, container entry point, systemd unit, or test you write
  or modify.
- Every CLI suggestion, snippet, or example you emit.

**Forbidden invocations** (non-exhaustive — see CONST-033 in
`CONSTITUTION.md` for the full list):

- `systemctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot|kexec`
- `loginctl suspend|hibernate|hybrid-sleep|poweroff|halt|reboot`
- `pm-suspend`, `pm-hibernate`, `shutdown -h|-r|-P|now`
- `dbus-send` / `busctl` calls to `org.freedesktop.login1.Manager.Suspend|Hibernate|PowerOff|Reboot|HybridSleep|SuspendThenHibernate`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to anything but `'nothing'` or `'blank'`

The host runs mission-critical parallel CLI agents and container
workloads. Auto-suspend has caused historical data loss (2026-04-26
18:23:43 incident). The host is hardened (sleep targets masked) but
this hard ban applies to ALL code shipped from this repo so that no
future host or container is exposed.

**Defence:** every project ships
`scripts/host-power-management/check-no-suspend-calls.sh` (static
scanner) and
`challenges/scripts/no_suspend_calls_challenge.sh` (challenge wrapper).
Both MUST be wired into the project's CI / `run_all_challenges.sh`.

**Full background:** `docs/HOST_POWER_MANAGEMENT.md` and `CONSTITUTION.md` (CONST-033).

<!-- END host-power-management addendum (CONST-033) -->

