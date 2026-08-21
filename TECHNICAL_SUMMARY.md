# Technical Summary: docker-compose-stack-backup-restore

> A Go-based CLI tool for backing up, encrypting, and restoring Docker Compose stacks — including filesystem content and named Docker volumes.

---

## 1. Project Overview

| Attribute | Value |
|---|---|
| **Language** | Go 1.24.3 |
| **Module path** | `github.com/crashlooping/docker-compose-stack-backup-restore` |
| **Binary name** | `dcsbr` |
| **External deps** | `github.com/goccy/go-yaml` (YAML config parsing) |
| **License** | MIT |
| **Platforms** | Cross-platform (Windows, Linux, ARM64) — prebuilt binaries included |

---

## 2. Architecture & Package Layout

```
cmd/dcsbr/              # CLI entry point — subcommand dispatch
├── main.go             # backup, restore, decrypt, verify commands
└── main_test.go        # Integration-style tests for CLI helpers

internal/
├── backup/             # Core orchestration: backup, restore, config
│   ├── backup.go       # BackupComposeStack, retention/pruning logic
│   ├── config.go       # Config struct, YAML loading, validation
│   ├── restore.go      # RestoreFromBackup — extraction + volume restore
│   ├── backup_test.go
│   ├── config_test.go
│   └── restore_test.go
├── archive/            # Low-level archive & crypto operations
│   ├── helpers.go      # Tar/GZ, Zip, encryption, extraction, CopyDir
│   └── helpers_test.go
└── docker/             # Docker Compose interaction layer
    ├── helpers.go      # Compose file detection, stack lifecycle, volume ops
    └── helpers_test.go
```

---

## 3. CLI Commands

### `backup [<source>]`

- Reads `config.yaml` from the working directory.
- Iterates over all configured `sources` (or a single source if specified).
- For each source:
  1. Detects the Docker Compose file (`docker-compose.yml`/`.yaml`).
  2. Stops the stack if running (graceful `docker compose down`).
  3. Checks filesystem readability (permission preflight).
  4. Exports all named Docker volumes to temporary tarballs.
  5. Creates archive(s) in configured formats (`tar.gz`, `zip`, or both) — **concurrently** via goroutines.
  6. Optionally encrypts each archive with AES-256-GCM (password ≥ 16 chars).
  7. Enforces backup retention (prunes oldest files beyond `max_backups`).
  8. Restarts the stack if it was originally running.

### `restore --target <dir> <archive>`

- Loads config for prefix/password.
- Extracts the stack name from the archive filename via regex.
- Prompts for user confirmation before proceeding.
- If the archive is `.enc`, decrypts it first (password from config or stdin prompt).
- Extracts the archive (tar.gz or zip) to a temp directory.
- Copies stack folders to the target directory.
- Restores any Docker volumes found in a `volumes/` subdirectory inside the archive.

### `decrypt --target <dir> <archive.enc>`

- Standalone decryption of an encrypted backup file.
- Password from config or stdin prompt.
- Outputs the decrypted archive to the target folder with the original naming convention.

### `verify`

- Loads and validates `config.yaml`.
- Prints the config with the password field masked (`*****`).
- Exits with an error if config is invalid.

---

## 4. Core Features

### 4.1 Multi-Format Archiving

Both `tar.gz` and `zip` formats are supported. Archives include:

- All files from the source directory (excluding `.git/` directories and Unix sockets/pipes/devices).
- Named Docker volumes exported as individual `.tar` files, stored under a `volumes/` prefix inside the archive.

Archive creation runs in parallel goroutines for efficiency.

Archive filenames use nanosecond-precision timestamps (`20060102_150405.000000000`) to prevent collisions from rapid sequential backups.

### 4.2 Docker Volume Backup & Restore

- **Discovery**: `docker compose config --format json` lists named volumes.
- **Export**: A temporary Alpine 3 container mounts the volume (read-only) and creates a tarball via `tar cf`.
- **Restore**: A temporary Alpine 3 container mounts the target volume and extracts the tarball via `tar xf`.
- Volume mount paths are heuristically parsed from the compose file (line-by-line scanning), falling back to `/volume`.

### 4.3 Authenticated Encryption (AES-256-GCM)

- Uses **AES-256-GCM** (authenticated encryption) — migrated from the original unauthenticated AES-CFB.
- Key derivation: `SHA-256(password)` → 32-byte AES key.
- Random 12-byte nonce per encryption, prepended to the ciphertext.
- Decryption returns a clear error on wrong password or corrupted data (GCM authentication tag verification).
- Encrypted files get a `.enc` extension; the original unencrypted file is removed after encryption.

### 4.4 Backup Retention (Pruning)

- Configurable via `max_backups` (default: 10, `0` = unlimited).
- After each backup run, files matching the pattern `<prefix>_backup_<stack>_*.<format>` are counted.
- If the count exceeds `max_backups`, the oldest files are deleted (sorted reverse-lexicographically by path).
- Retention is applied per stack, per format, and separately for encrypted vs. unencrypted files.

### 4.5 Path Traversal Protection

- `safePath(dest, entryPath)` validates that the resolved path stays within the destination directory.
- Used by `ExtractTarGz`, `ExtractZip`, and `ExtractTar` — prevents Zip Slip / Tar Slip attacks.

### 4.6 Config Validation

The `validate()` method enforces:

| Field | Rule |
|---|---|
| `prefix` | Required |
| `formats` | At least one, must be `tar.gz` or `zip` |
| `target` | Required |
| `password` | Optional; if set, must be ≥ 16 characters |
| `max_backups` | Negative values are reset to 0 (unlimited) |

---

## 5. Security Considerations

| Area | Implementation |
|---|---|
| **Encryption** | AES-256-GCM (AEAD) — authenticated, wrong-password detectable |
| **Nonce** | 12 random bytes per encryption via `crypto/rand` |
| **Key derivation** | SHA-256 (single hash) — no PBKDF2/bcrypt/Argon2 |
| **Archive extraction** | `safePath()` prevents path traversal |
| **Config secrets** | Password masked in `verify` output |
| **Temp files** | Cleaned up after encryption and restore |
| **Sudo check** | Linux-only: aborts if `sudo_required: true` and not root |

> **Note**: Key derivation uses a single SHA-256 hash rather than a slow KDF. For production use with weak passwords, consider adding PBKDF2 or Argon2id.

---

## 6. Error Handling Patterns

- **Config loading**: Errors are propagated and cause CLI exit with code 1.
- **Volume export failures**: Non-fatal — prints a warning and continues without volumes.
- **Cleanup failures**: Logged to stderr but do not abort the operation.
- **Docker command failures**: Stack-state probe treats errors as "not running" (best-effort).
- **Restore**: Pre-validates archive existence; decrypt errors are fatal.

---

## 7. Open Issues & Known Gaps

| Issue | Priority | Description |
|---|---|---|
| [#25](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/25) | Medium | `GetVolumeMountPathFromCompose` uses fragile line-by-line text scanning |
| [#30](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/30) | Low | Add `list` command to show available backups |
| [#31](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/31) | Low | Add `--config` flag to specify config path |
| [#32](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/32) | Low | Add `--dry-run` mode |
| [#33](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/33) | Low | Add backup integrity verification (checksums) |
| [#34](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/34) | Test | Deduplicate tests and add missing coverage |
| [#35](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/35) | Low | Mask password input during prompts |
| [#36](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/36) | Low | Add `--verbose` / `--quiet` flags |
| [#37](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/37) | Low | Remove unused `DockerComposeCmd` constant |
| [#38](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/38) | Low | Make Alpine 3 volume image configurable |
| [#39](https://github.com/crashlooping/docker-compose-stack-backup-restore/issues/39) | Low | Add `--version` flag |

---

## 8. Test Coverage

- **46 tests, all passing** (as of last verification).
- Packages tested: `cmd/dcsbr`, `internal/backup`, `internal/archive`, `internal/docker`.
- Tests cover: archive creation (tar.gz, zip), `.git` exclusion, Unix socket skipping, encryption/decryption, config validation, CLI helpers, Docker compose file detection, volume mount path parsing, and backup retention pruning.
- No integration tests requiring a live Docker daemon (volume export/restore paths are untested in CI).

---

## 9. Build & Run

```sh
# Build
go build -o dcsbr.exe ./cmd/dcsbr

# Run
go run ./cmd/dcsbr

# Test
go test ./...
```