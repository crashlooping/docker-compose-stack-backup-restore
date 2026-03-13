# Code Analysis Summary: docker-compose-stack-backup-restore (Verified)

## Verification Snapshot

- Date: 2026-03-13
- Test status: `go test ./...` -> **PASS**
- Last result after test fixes: **30 passed, 0 failed**
- Static checks: `go vet ./...` reported no diagnostics in this review pass.

---

## What Was Fixed First (Tests)

The previously failing tests in `internal/archive/helpers_test.go` were fixed:

1. Added missing `defer f.Close()` in archive-read tests to avoid Windows temp-dir cleanup failures.
2. Made Docker volume export test deterministic across environments by accepting either:
   - a runtime error from Docker, or
   - a successful export that produces a valid output file.

These changes resolved the three failing tests from the prior run.

---

## Re-Verified Findings (Severity Ranked)

## Critical

### 1. Archive extraction path traversal risk (Zip Slip / Tar Slip)

**Location**: `internal/archive/helpers.go`

Extraction joins archive entry names directly into destination paths without validating that the resulting path remains inside the target directory.

- `ExtractTarGz`: `path := filepath.Join(dest, hdr.Name)`
- `ExtractZip`: `path := filepath.Join(dest, f.Name)`
- `ExtractTar`: `path := filepath.Join(dest, hdr.Name)`

**Impact**: A crafted archive can write files outside the intended restore directory.

**Recommendation**: Clean + validate each target path (`filepath.Clean`) and reject entries that escape destination root.

---

### 2. Encryption is unauthenticated; wrong-password detection is unreliable

**Location**: `internal/archive/helpers.go`

Current implementation uses AES-CFB without authentication (no MAC/AEAD). CFB decryption can produce output even with wrong passwords and may not return an error.

**Impact**: Silent corruption risk; restore may appear successful with invalid plaintext.

**Recommendation**: Migrate to AEAD (AES-GCM or XChaCha20-Poly1305) with authentication tag verification.

---

## High

### 3. Silent error handling in operational paths

**Location**: `internal/backup/backup.go`, `internal/docker/helpers.go`

- Multiple `os.Remove(...)` calls ignore errors.
- Docker stack-state probe masks execution failure as "not running":
  - `return false, nil // treat as not running if error`

**Impact**: Hidden operational failures, missed cleanup, confusing behavior.

**Recommendation**: Capture and log cleanup failures; propagate or classify Docker command errors explicitly.

---

### 4. Restore preflight and error handling gaps

**Location**: `cmd/dcsbr/main.go`

- No early archive existence/type pre-validation before prompting and restore flow.
- Encrypted restore path reloads config with ignored error:
  - `cfg, _ := backup.LoadConfig("config.yaml")`

**Impact**: Late failure discovery and reduced debuggability.

**Recommendation**: Pre-validate archive file + extension/magic, and never ignore config-load errors.

---

## Medium

### 5. CLI parsing inconsistency

**Location**: `cmd/dcsbr/main.go`

`backup` manually parses positional args while `restore`/`decrypt` use `flag` sets.

**Impact**: Inconsistent UX and weaker subcommand help ergonomics.

**Recommendation**: Normalize all subcommands around `flag.FlagSet` patterns.

---

### 6. Fragile Compose mount path extraction

**Location**: `internal/docker/helpers.go`

Mount path detection parses compose YAML by line scanning and string splitting.

**Impact**: Incorrect mount detection for more complex compose syntax.

**Recommendation**: Parse compose content via structured YAML model or derive mount metadata via Docker APIs.

---

## Low / Process

### 7. Test suite quality improved, but broader coverage is still advisable

Core test failures were fixed and suite is green, but integration scenarios (full backup+restore with real Docker volumes and corrupted encrypted payloads) should be expanded.

---

## Corrections From Previous Draft

1. "Mask password in verify output" is already implemented.
2. "Encryption cleanup race condition" is better classified as cleanup/error-handling robustness, not a race.
3. Two critical issues were missing before and are now included:
   - extraction path traversal,
   - unauthenticated encryption integrity risk.

---

## Recommended Next Steps

1. Fix path traversal guards in all extract functions first.
2. Replace CFB encryption with authenticated encryption (AEAD) and add migration strategy for old backups.
3. Harden error handling (`os.Remove`, docker command failures, config load) with explicit logging and propagation.
4. Add restore preflight validation.
5. Add focused integration tests for archive safety and wrong-password behavior.
