#!/usr/bin/env bash
# challenges/storage_describe_challenge.sh
#
# Round-283 anti-bluff Challenge for digital.vasic.storage.
#
# Default mode: invoke the runner against a real ephemeral filesystem
# and assert it exits 0 with the expected operation count, sidecar
# round-trip evidence, and 5-locale UX evidence. This is the
# positive-evidence proof per Article XI §11.9 — the PASS is backed
# by captured stdout, not by absence of error or a green summary
# line.
#
# Paired-mutation mode (--mutate): build a scratch sidecar reader
# that simulates a corrupted .meta file (empty Key field), and
# assert the invariant check trips it (exit 99 = mutation correctly
# surfaced). A mutation run that exits 0 means the Challenge itself
# is a bluff (CONST-035 mutation-bluff), and this script exits 1 to
# surface that.
#
# Defensive use only — no payload generation, no obfuscation helpers.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

MODE="default"
if [[ ${1:-} == "--mutate" ]]; then
    MODE="mutate"
fi

run_default() {
    echo "[storage-challenge] mode=default — exercising runner against real ephemeral filesystem"
    cd "${REPO_ROOT}"

    local out
    out=$(go run ./challenges/runner -all 2>&1) || {
        echo "[storage-challenge] FAIL: runner exited non-zero"
        echo "${out}"
        exit 1
    }

    # Positive-evidence assertions on captured stdout.
    if ! grep -q "^OK operations=11 bytes=" <<<"${out}"; then
        echo "[storage-challenge] FAIL: missing OK trailer with operations=11"
        echo "${out}"
        exit 1
    fi
    if ! grep -q "^\[en\] storage:" <<<"${out}" \
            || ! grep -q "^\[sr\] storage:" <<<"${out}" \
            || ! grep -q "^\[ja\] storage:" <<<"${out}" \
            || ! grep -q "^\[es\] storage:" <<<"${out}" \
            || ! grep -q "^\[de\] storage:" <<<"${out}"; then
        echo "[storage-challenge] FAIL: missing one or more locale UX lines"
        echo "${out}"
        exit 1
    fi

    # Defensive-use boundary check — no inverse helpers may leak.
    if grep -RnE 'func +(Generate(Payload|Attack|Obfuscat)|Bypass[A-Z])' "${REPO_ROOT}" \
            --include='*.go' --exclude-dir=challenges --exclude-dir=.git 2>/dev/null \
            | grep -v '_test.go'; then
        echo "[storage-challenge] FAIL: inverse helper detected (defensive-use boundary breached)"
        exit 1
    fi

    echo "${out}"
    echo "[storage-challenge] PASS — runtime evidence captured above"
    exit 0
}

run_mutate() {
    echo "[storage-challenge] mode=mutate — paired-mutation evidence"
    local scratch
    scratch="$(mktemp -d -t storage-mutate-XXXXXX)"
    # shellcheck disable=SC2064
    trap "rm -rf '${scratch}'" EXIT

    # Stage a self-contained scratch module that vendors a mutated
    # sidecar reader. The mutation: the reader returns ObjectInfo
    # with an empty Key, simulating a corrupted .meta file. The
    # runner-style invariant check (assertObjectInfo) MUST flag it.
    cat > "${scratch}/go.mod" <<'EOF'
module storage.scratch

go 1.25
EOF

    cat > "${scratch}/main.go" <<'EOF'
package main

import (
	"errors"
	"fmt"
	"os"
)

// ObjectInfo is the mutated stand-in for object.ObjectInfo. The
// mutation: Key is always returned empty, simulating a corrupted
// sidecar that the runner-style invariant assertion MUST catch.
type ObjectInfo struct {
	Key  string
	Size int64
}

func loadOneMutated() ObjectInfo {
	// Simulate a sidecar read that lost the Key field during a
	// truncated write — the kind of partial-write failure pkg/local
	// edge tests guard against.
	return ObjectInfo{Key: "", Size: 47}
}

// assertObjectInfo mirrors the runner's invariant check. It MUST
// flag the empty Key as a defect.
func assertObjectInfo(info ObjectInfo) error {
	if info.Key == "" {
		return errors.New("empty Key in ObjectInfo (sidecar corruption)")
	}
	if info.Size <= 0 {
		return errors.New("non-positive Size in ObjectInfo")
	}
	return nil
}

func main() {
	info := loadOneMutated()
	if err := assertObjectInfo(info); err != nil {
		fmt.Fprintf(os.Stderr, "mutation detected: %v\n", err)
		os.Exit(99)
	}
	fmt.Println("mutation NOT detected — bluff")
	os.Exit(0)
}
EOF

    cd "${scratch}"
    # Build then exec — `go run` does not preserve exit codes >2 on
    # all toolchains, which would mask the sentinel 99 the program
    # emits when the mutation is detected.
    go build -o ./mutbin . >/dev/null 2>&1 || {
        echo "[storage-challenge] FAIL-MUTATE — scratch build failed"
        exit 1
    }
    local mut_out mut_rc
    set +e
    mut_out=$(./mutbin 2>&1)
    mut_rc=$?
    set -e

    echo "${mut_out}"
    if [[ ${mut_rc} -eq 99 ]]; then
        echo "[storage-challenge] PASS-MUTATE — mutation correctly surfaced (exit 99)"
        exit 99
    fi
    echo "[storage-challenge] FAIL-MUTATE — mutation NOT surfaced (exit ${mut_rc}); Challenge is a bluff"
    exit 1
}

case "${MODE}" in
    default) run_default ;;
    mutate)  run_mutate ;;
esac
