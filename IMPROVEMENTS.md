# Future Improvements

Potential features and enhancements for consideration, ordered by the four topics below.

---

## 1. Notifications

Send backup/restore results through one or more channels so operators don't need to watch logs.

### General Approach

Decouple notification delivery from backup logic via a lightweight interface:

```go
type Notifier interface {
    Send(ctx context.Context, subject string, body string) error
}
```

Each channel implements the interface. The `backup` and `restore` commands call `Notifier.Send()` on success and on failure, passing a short summary (stack name, format, duration, error if any).

### Channel Ideas

| Channel | Library / Approach | Notes |
|---|---|---|
| **Email (SMTP)** | `net/smtp` (stdlib) or `gomail` | Simple SMTP; no extra dep if using stdlib |
| **Telegram** | [nikoksr/notify](https://github.com/nikoksr/notify) | Single lib covering Telegram, Slack, Discord, Pushover, and more |
| **Webhook** | `net/http` (stdlib) | Generic JSON POST to any endpoint |
| **Desktop (Windows)** | `golang.org/x/sys/windows` or `toast` | Windows toast notifications for local runs |

### Config Sketch

```yaml
notifications:
  on_success: false        # only notify on failure by default
  on_failure: true
  telegram:
    token: "..."
    chat_id: "..."
  email:
    smtp_host: smtp.example.com
    smtp_port: 587
    from: dcsbr@example.com
    to: admin@example.com
```

### Considerations

- Keep the interface synchronous and simple — retries and backoff belong in the implementation.
- Credentials belong in environment variables or a secrets file, not in `config.yaml` checked into git.
- `nikoksr/notify` is a good single-dependency choice if multiple chat channels are wanted; for email-only, stdlib suffices.

---

## 2. Built-in Scheduler

Run backups on a timer without relying on an external cron or systemd timer.

### Motivation

- Windows users don't always have Task Scheduler configured for WSL/Go binaries.
- A self-contained scheduler simplifies deployment: one binary, no crontab.
- Enables time-of-day-aware retention (daily/weekly/monthly — see §4).

### Approach

Embed a lightweight cron-like scheduler using [`robfig/cron`](https://github.com/robfig/cron) (the de-facto Go scheduling library):

```yaml
scheduler:
  enabled: true
  schedule: "0 2 * * *"   # daily at 2 AM (standard cron expression)
  # schedule: "@daily"    # or predefined entries
  # schedule: "0 2 * * 0" # weekly on Sunday
```

When `scheduler.enabled` is `true` and the subcommand is `run` (or no subcommand is given), the tool stays running and executes backups on the cron schedule.

### CLI Sketch

```sh
# Run once (current behavior, no change)
dcsbr backup

# Start scheduler daemon
dcsbr daemon

# Or: if scheduler is enabled in config, plain invocation starts the daemon
dcsbr
```

### Considerations

- Adds `github.com/robfig/cron/v3` as a dependency (~2.5k stars, stable).
- The daemon should handle graceful shutdown (SIGINT/SIGTERM).
- Log output should include timestamps when running as a daemon.
- A `--oneshot` flag could override the scheduler and run once regardless.

---

## 3. Non-Interactive Mode (No Confirmation Prompts)

### Current State

The `restore` command prompts `Are you sure you want to proceed? (y/N)` and waits for stdin. This blocks automation/scripting.

### Proposed Flags

```sh
# Skip all confirmation prompts
dcsbr restore --target ./restore backup.tar.gz --yes

# Or short form
dcsbr restore --target ./restore backup.tar.gz -y
```

### Implementation

- Add `--yes` / `-y` flag to `restore` (and `decrypt` if it ever prompts).
- When set, skip the confirmation prompt and proceed immediately.
- Also skip the password prompt for encrypted archives if `--yes` is passed without a configured password — fail fast instead of blocking on stdin.

### Config Override

```yaml
backup:
  non_interactive: true   # global default for all commands
```

### Considerations

- `--yes` should **not** suppress error messages, only interactive prompts.
- Useful for CI/CD pipelines, cron jobs, and the built-in scheduler (§2).

---

## 4. Time-Aware Retention (Daily / Weekly / Monthly)

### Current State

Retention is a simple count (`max_backups`): keep the N newest files, delete the rest. This works but doesn't preserve historical coverage — a burst of backups can push out all older copies.

### Proposed Model

Replace the flat count with a tiered retention policy:

```yaml
backup:
  retention:
    daily:   7    # keep 1 backup per day for the last 7 days
    weekly:  4    # keep 1 backup per week for the last 4 weeks
    monthly: 6    # keep 1 backup per month for the last 6 months
```

### How It Works

1. After a backup completes, collect all existing backup files for that stack/format.
2. Group files by the period they fall into (day, week, month) based on the timestamp embedded in the filename (`YYYYMMDD_HHMMSS`).
3. For each period, keep the newest file and mark the rest as eligible for deletion.
4. Delete all eligible files that are not protected by any tier.

### Example

With `daily: 7, weekly: 4, monthly: 6`:

- The 7 most recent daily backups are kept (one per calendar day).
- The 4 most recent weekly backups are kept (one per ISO week, may overlap with daily).
- The 6 most recent monthly backups are kept (one per calendar month, may overlap).
- Anything outside all windows is deleted.

### Backward Compatibility

- If `retention` is not set, fall back to the existing `max_backups` behavior.
- If both `retention` and `max_backups` are set, `retention` takes precedence (with a log message).

### Considerations

- Requires parsing the timestamp from the filename — already done for `extractStackNameFromArchive`.
- A file can satisfy multiple tiers (e.g., the newest backup is always kept by all three).
- The `max_backups = 0` (unlimited) behavior should also apply when all retention values are 0.
- Works best with the built-in scheduler (§2) so backups happen at predictable times.

---

## Appendix: Dependency Impact Summary

| Feature | Suggested Library | Size / Notes |
|---|---|---|
| Notifications | [`nikoksr/notify`](https://github.com/nikoksr/notify) | Pulls in several sub-deps; single import for multi-channel |
| Scheduler | [`robfig/cron/v3`](https://github.com/robfig/cron) | Lightweight, pure Go, no CGO |
| Non-interactive | None (stdlib `flag`) | No new dependency |
| Tiered retention | None (stdlib `time`, `sort`) | No new dependency |