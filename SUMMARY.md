# Code Analysis Summary: docker-compose-stack-backup-restore

## Project Overview

**Purpose**: Go application for backing up Docker Compose stacks with the following features:

- Support for tar.gz and zip backup formats
- Optional AES-256 encryption
- Docker volume backup and restoration
- Automated retention policy (max_backups)
- Support for encrypted backup files

---

## 🔴 Critical Issues

### 1. **Weak Error Handling Throughout Codebase**

**Location**: Multiple files (`backup.go`, `helpers.go`)

**Problem**: Errors are silently ignored in several critical places:

- Volume export failures only produce warnings and continue backup
- `os.Remove()` calls never check if deletion succeeded
- Docker command failures in `IsComposeStackRunning` treated as "not running"
- Permission check failures silently skip problematic files

**Example** (`backup.go`, line ~180):

```go
for _, f := range volumeTarballs {
    fmt.Printf("Removing temp file: %s\n", f)
    os.Remove(f)  // ❌ Error ignored - temp files leak on failure
}
```

**Impact**:

- Backups silently incomplete with missing volumes
- Temporary files accumulate on disk
- No way to detect which operations failed

**Recommendation**: Wrap every error and decide whether to fail-fast or continue with explicit logging.

---

### 2. **Encryption Cleanup Race Condition**

**Location**: `backup.go`, `BackupComposeStackWithFormats` function

**Problem**: If encryption fails after backup creation, the unencrypted backup file may persist:

```go
if password != "" {
    for _, format := range formats {
        encPath := backupPath + ".enc"
        err := archive.EncryptFile(backupPath, encPath, password)
        if err != nil {
            return fmt.Errorf("failed to encrypt backup: %w", err)
        }
        os.Remove(backupPath)  // ❌ Deleted only after encryption succeeds
    }
}
```

**Impact**: Sensitive unencrypted data left on disk if encryption fails.

**Recommendation**: Delete unencrypted file immediately after successful encryption, inside the same critical section.

---

### 3. **No Backup Integrity Verification**

**Location**: `backup.go`, `BackupComposeStackWithFormats` function

**Problem**: After creating a backup, no verification that:

- Archive is valid and readable
- All expected files are included
- Archive can be restored successfully

**Impact**: Corrupted backups discovered only during restore (when data is needed most).

**Recommendation**:

- Quick verify: List archive contents immediately after creation
- Full verify: Optional extraction to temp directory for validation

---

### 4. **Missing Configuration Validation**

**Location**: `config.go`

**Problem**: Only `prefix` field validated. Missing validation for:

- Empty `sources` list
- Invalid/inaccessible `target` path
- At least one `format` specified
- Password minimum length (16 chars only checked at encryption time)

**Current Code**:

```go
func LoadConfig(path string) (*Config, error) {
    // ... only checks MaxBackups
    if cfg.Backup.MaxBackups <= 0 {
        cfg.Backup.MaxBackups = 10
    }
    return &cfg, nil
}
```

**Impact**: Errors discovered during backup execution instead of startup.

**Recommendation**: Add comprehensive validation function called immediately after config load.

---

### 5. **CLI Argument Parsing Inconsistency**

**Location**: `cmd/dcsbr/main.go`

**Problem**:

- `backup` command uses manual index checking: `os.Args[2]`
- `restore` and `decrypt` commands use properly structured `flag` package
- No support for subcommand help (e.g., `dcsbr backup --help`)

**Code**:

```go
if len(os.Args) > 2 {
    sourceArg := os.Args[2]  // ❌ Direct indexing, fragile
}
```

**Impact**: Inconsistent user experience, no help for subcommands.

**Recommendation**: Migrate `backup` command to use `flag` package for consistency.

---

### 6. **Insecure Password Handling**

**Location**: `config.go`, `cmd/dcsbr/main.go`

**Problem**:

- Passwords stored in plain-text YAML files
- Passwords read into memory and kept there during entire operation
- No protection from memory dumps or core dumps
- Can appear in shell history if entered interactively

**Impact**: Compromise of backup encryption key if system is breached or credentials stolen.

**Recommendation**:

- Use environment variables: `DCSBR_PASSWORD`
- Support external secret managers (HashiCorp Vault, 1Password CLI)
- Clear password from memory after use
- Never log password or config dump with unmasked password

---

### 7. **Fragile Docker Compose YAML Parsing**

**Location**: `internal/docker/helpers.go`, `GetVolumeMountPathFromCompose` function

**Problem**: Uses naive string splitting instead of proper YAML parsing:

```go
if strings.Contains(line, volume+":") {
    parts := strings.SplitN(line, ":", 2)
    candidate := strings.TrimSpace(parts[1])
    // ❌ Breaks when:
    // - Values are quoted: "path: /mnt:/data"
    // - Multiple colons: "path:/data:ro"
    // - Complex YAML structures
}
```

**Impact**:

- Fails on realistic Docker Compose files with quoted strings
- Exports wrong volume paths
- Silent fallback to `/volume` could backup wrong data

**Recommendation**: Parse compose file as proper YAML instead of text lines.

---

### 8. **Concurrent Job Error Handling**

**Location**: `backup.go`, `runArchiveJobs` function

**Problem**:

```go
func runArchiveJobs(jobs []func() error) error {
    var wg sync.WaitGroup
    errCh := make(chan error, len(jobs))
    for _, job := range jobs {
        wg.Add(1)
        go func(j func() error) {
            defer wg.Done()
            errCh <- j()  // ❌ Will return only first error, others continue
        }(job)
    }
    // ...
}
```

**Issues**:

- If first job fails, others still run (could overwrite files)
- No cleanup of partial results
- No panic recovery in goroutines

**Impact**: Corrupted or incomplete backups in concurrent scenarios.

**Recommendation**: Add channel monitoring to stop other jobs when first error occurs, plus panic recovery.

---

## 🟠 High Priority Issues

### 9. **No Audit Logging**

**Problem**: Only uses `fmt.Print` to stdout/stderr

- No persistent logs
- Can't track backup history
- No record of failures for debugging
- Can't prove compliance/audit trail

**Recommendation**: Implement structured logging to file (JSON format):

```json
{
  "timestamp": "2024-03-13T10:30:45Z",
  "event": "backup_completed",
  "stack": "authentik",
  "format": "tar.gz",
  "size_bytes": 1234567,
  "duration_sec": 123,
  "encrypted": true,
  "status": "success"
}
```

---

### 10. **Missing Validation of Archive Before Restore**

**Location**: `cmd/dcsbr/main.go`, restore command

**Problem**: Doesn't validate archive exists or is readable before starting restore process.

**Impact**: User waits for extract to start before discovering file issues.

**Recommendation**: Pre-validate archive file exists, is readable, and has correct magic bytes.

---

### 11. **No Help for Subcommands**

**Problem**: `dcsbr backup --help` fails, only `dcsbr --help` works.

**Impact**: Users can't get help for specific commands.

**Recommendation**: Each subcommand should have help text.

---

### 12. **Incomplete Encryption Passthrough in Restore**

**Location**: `cmd/dcsbr/main.go`, restore command

**Problem**:

```go
opts := backup.RestoreOptions{TargetDir: resolvedTarget}
if strings.HasSuffix(archivePath, ".enc") {
    cfg, _ := backup.LoadConfig("config.yaml")  // ⚠️ Error ignored
}
```

If LoadConfig fails, password is silently empty and user will be prompted instead.

---

### 13. **No Progress Reporting for Large Backups**

**Problem**: For multi-gigabyte stacks, no indication of:

- Current progress
- Estimated time remaining
- Bytes processed

**Impact**: User sees frozen screen, doesn't know if process is hanging.

---

### 14. **Symlink Handling Undefined**

**Location**: `internal/archive/helpers.go`

**Problem**: Archive functions don't explicitly handle symlinks. They may be:

- Followed (leaking data outside intended scope)
- Skipped (incomplete backup)
- Changed to regular files (data duplication)

**Recommendation**: Add explicit symlink policy (skip, follow, or preserve).

---

## 🟡 Medium Priority Issues

### 15. **Minimal Test Coverage**

**Problem**: Limited test suite missing:

- Encryption/decryption round-trip
- Volume export and restore
- Config loading and validation
- Error scenarios
- Archive integrity
- Concurrent backup scenarios

**Impact**: Regressions not caught, reduced reliability.

**Recommendation**: Target >80% code coverage with integration tests.

---

### 16. **No Dry-Run Mode**

**Problem**: Can't preview what will be backed up without actually running backup.

**Recommendation**: Add `--dry-run` flag to show:

- What sources will be backed up
- Estimated size
- What volumes will be included
- Where backup will be stored

---

### 17. **Backup Rotation Not Atomic**

**Location**: `backup.go`, `cleanupBackupsAfterRun`

**Problem**: Old backups deleted after new one created. If process crashes between steps:

- New backup created but old one not deleted: waste space
- Old backup deleted but new one incomplete: data loss

**Recommendation**: Use atomic file operations or transaction-like semantics.

---

### 18. **No Resource Limits**

**Problem**: Large backups could:

- Consume all disk space
- Exhaust available memory
- Timeout on slow systems

**Recommendation**: Add configurable limits:

```yaml
backup:
  max_backup_size_gb: 100
  timeout_minutes: 60
  memory_limit_mb: 2048
```

---

### 19. **Default Permissions Too Open**

**Location**: `archive/helpers.go`, `CopyDir` function

**Problem**: Uses `info.Mode()` which preserves original permissions. If source has world-readable sensitive data, backup will too.

**Recommendation**: Apply restrictive mask: `0o600` for files, `0o700` for directories.

---

### 20. **Inconsistent Prefix Handling**

**Location**: `backup.go`, patterns

**Problem**:

```go
const (
    backupTarGzPattern = "%s_backup_%s_%s.tar.gz"  // Uses %s but...
)
// Later hardcodes in other places:
if strings.HasPrefix(outName, cfg.Backup.Prefix+"_backup_") {
    // Prefix handling inconsistent
}
```

**Impact**: Confusing code, potential bugs if pattern format changes.

---

## ✅ Positive Aspects

- ✅ Good package separation (archive, docker, backup packages)
- ✅ AES-256 encryption uses Go standard library (secure)
- ✅ Automatic git directory exclusion during backup
- ✅ Intelligent stack stop/restart during backup
- ✅ Multiple format support (tar.gz, zip)
- ✅ Retention policy implemented
- ✅ Support for both encrypted and non-encrypted backups
- ✅ Config-driven approach

---

## 📋 Prioritized Improvement Plan

### Phase 1: Critical (Implement First)

1. **Add comprehensive error handling** - wrap all errors, decide fail-fast vs continue
2. **Config validation on load** - all required fields, valid paths, password length
3. **Backup integrity verification** - quick list + optional full extraction verify
4. **Fix encryption cleanup** - atomic deletion after successful encryption
5. **Structured logging** - JSON logs to file with all important events

**Estimated effort**: 4-6 hours

---

### Phase 2: High Priority (Implement Next)

1. Parse compose files properly as YAML, not text
2. Implement audit logging to file
3. Unify CLI to use flag package consistently
4. Move password to environment variables
5. Improve concurrent job error handling and goroutine panic recovery
6. Add pre-restore archive validation

**Estimated effort**: 6-8 hours

---

### Phase 3: Medium Priority (Polish)

1. Expand test coverage to >80%
2. Add dry-run mode with `--dry-run` flag
3. Add progress reporting for large backups
4. Add resource limits to config
5. Improve help/documentation for all subcommands
6. Better symlink handling strategy
7. Make backup rotation atomic

**Estimated effort**: 8-10 hours

---

## 🚀 Quick Wins (Can Do Immediately)

| Task | Time | Impact |
|------|------|--------|
| Fix `os.Remove()` error handling | 5 min | Reduces file leaks |
| Add config validation | 10 min | Catches errors early |
| Add help for subcommands | 10 min | Better UX |
| Mask password in verify output | 5 min | Security |
| Add missing error checks | 15 min | Reliability |

Total: ~45 minutes for significant improvements

---

## Security Considerations

1. **Encryption**: AES-256-CFB is secure but consider authenticated encryption (GCM) for future
2. **Passwords**: Move from YAML to environment variables immediately
3. **Permissions**: Ensure backup files created with restrictive perms (0o600)
4. **Audit Trail**: Implement logging to detect unauthorized access attempts
5. **Temp Files**: Ensure secure cleanup of temporary files (consider secure overwrite)

---

## Recommendations for Next Steps

1. **Start with Phase 1** - these are safety-critical
2. **Run tests before and after** each change
3. **Add integration tests** for complete backup/restore cycles
4. **Document configuration** thoroughly
5. **Create troubleshooting guide** for common issues
6. **Consider adding** dry-run mode early (useful for testing)
