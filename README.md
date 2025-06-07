# docker-compose-stack-backup-restore 🚀

Easily back up, encrypt, and restore your Docker Compose stacks—including all volumes and filesystems—with a single tool!

---

## 🔒 How encryption works

- If you set a `password` in your `config.yaml`, all backup files (`.tar.gz` and `.zip`) are encrypted automatically after creation using AES-256 encryption.
- Encrypted backups are saved with an additional `.enc` extension (e.g., `backup_stack_20250521_123456.tar.gz.enc`).
- The original, unencrypted backup file is deleted after encryption for safety.
- To restore or decrypt, the tool will use the password from your config, or prompt you to enter it if not set.
- Encryption and decryption are performed locally using Go's standard library—no external tools or services are required.
- Only someone with the correct password can decrypt and restore your backup files.
- For strong security, your password **must be at least 16 characters long**. Shorter passwords are not accepted by the tool.

---

## 📦 Usage

### 1️⃣ Create your `config.yaml`

Copy the provided example file and adjust it to your needs:

```cmd
copy config.example.yaml config.yaml
```

Edit `config.yaml` to specify:

- 📁 The backup formats you want (`tar.gz`, `zip`, or both)
- 🗂️ One or more source folders to back up
- 🎯 The target folder where all backups will be stored
- 🔑 (Optional) A password for encryption (must be at least 16 characters)
- ♻️ (Optional) Maximum number of backups to retain (`max_backups`)
- **prefix**: Required. All backup files will be prefixed with this value (e.g., `dcsbr_backup_stackname_...`).

**Example `config.yaml`:**

```yaml
backup:
  formats: ["tar.gz", "zip"]
  sources:
    - ~/docker/authentik
    - ~/docker/uptime-kuma
  target: ~/backup
  password: your-very-strong-password-here # optional, must be >16 chars
  max_backups: 10 # optional, default is 10
  prefix: dcsbr # required, prefix for all backup files
  sudo_required: false # optional, default is false. Set to true if you need sudo to access the source directories
```

---

### 2️⃣ Run the tool

Build or run the tool from the project root:

```sh
# Build (Windows example)
go build -o dcsbr.exe ./cmd/dcsbr

# Or run directly
go run ./cmd/dcsbr
```

The tool will read your `config.yaml` and back up all specified stacks to the target folder in the selected formats. 🗃️

---

💡 **Tip:**  
You can adjust the config at any time to add/remove stacks, change backup formats, or set retention.

---

### 3️⃣ CLI Commands

- **Backup:**

  ```sh
  dcsbr.exe backup
  ```

  All backup files will be prefixed with the configured value (e.g., `dcsbr_backup_...`).

- **Backup a single source:**

  ```sh
  dcsbr.exe backup .develop/authentik
  ```

  Only the specified source from the `sources` list in your config.yaml will be backed up. If the source is not found, an error will be printed. If no source is specified, all sources will be backed up.

- **Restore:**

  ```sh
  dcsbr.exe restore --target <restore-folder> <backup-archive>
  ```

  - `<backup-archive>` must start with the configured prefix (e.g., `dcsbr_backup_...`).
  - The tool auto-detects the stack name from the archive and restores to `<restore-folder>/<stack-name>`.
  - Fails if the target folder already exists.
  - Prompts for confirmation before restoring.

- **Decrypt:**

  ```sh
  dcsbr.exe decrypt --target <target-folder> <backup-archive>
  ```

  - `<backup-archive>` must start with the configured prefix (e.g., `dcsbr_backup_...`).
  - The decrypted file will be written to `<target-folder>` with the `.enc` extension removed.
  - If you have a password in your config, it will be used automatically. Otherwise, you will be prompted for the password.

- **Verify config:**

  ```sh
  dcsbr.exe verify
  ```

  Prints and verifies the config.yaml file, masking the password field.

- **Help:**

  ```sh
  dcsbr.exe --help
  ```

  Shows all available commands and options.

---

### 4️⃣ Backup retention

- The tool supports automatic backup retention via the `max_backups` config option (default: 10).
- After each backup, the tool will prune the oldest backups in the target folder, keeping only the most recent `max_backups` for each stack.
- Set `max_backups` in your config to control retention.

---

### 5️⃣ Permissions & Docker notes

- The tool checks for unreadable files before stopping the stack and will warn you about permission errors.
- If you see permission errors, try running the tool with elevated privileges (e.g., `sudo` on Linux/Mac).
- All Docker operations use the `alpine:3` image for volume export/import.
- Empty folders are included in backups.
- In CI, Docker-dependent tests are skipped if Docker is not available.

---

### 6️⃣ Limitations & known issues

- Only supports Docker Compose stacks (not Swarm or Kubernetes).
- Requires Docker to be installed and accessible in your PATH.
- Password-based encryption uses AES-256; do not lose your password—backups cannot be decrypted without it.
- Retention is per stack and per format; manual cleanup may be needed for custom scenarios.

---

## 🛠️ Features

- Multi-format backup: `tar.gz` and/or `zip`
- Backs up both stack folders and all Docker volumes
- Supports multiple source stacks
- Cross-platform (Windows, Linux, macOS)
- Simple YAML configuration
- Fast, reliable, and easy to automate
- Password-based AES-256 encryption
- Backup retention with automatic pruning
- Includes empty folders and checks for permission errors

---

## 🤝 Contributing

Pull requests and issues are welcome! See the code, suggest improvements, or open an issue if you find a bug.

---

## 📄 License

MIT

---

## ℹ️ AI Assistance

This project was developed with the assistance and guidance of GitHub Copilot agent mode using GPT-4.1.

---
