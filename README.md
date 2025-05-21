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

**Example `config.yaml`:**

```yaml
backup:
  formats: ["tar.gz", "zip"]
  sources:
    - ~/docker/authentik
    - ~/docker/uptime-kuma
  target: ~/backup
  password: foobar # optional
```

---

### 2️⃣ Run the tool

Build or run the tool from the project root:

```cmd
REM Build (Windows example)
go build -o dcsbr.exe ./cmd/dcsbr

REM Or run directly
go run ./cmd/dcsbr
```

The tool will read your `config.yaml` and back up all specified stacks to the target folder in the selected formats. 🗃️

---

💡 **Tip:**  
You can adjust the config at any time to add/remove stacks or change backup formats.

---

### 3️⃣ Restore a backup

To restore a backup, use the following command:

```cmd
dcsbr.exe restore --target <restore-folder> <backup-archive>
```

- `<backup-archive>` can be a `.tar.gz`, `.zip`, or an encrypted `.enc` file.
- If the backup is encrypted (ends with `.enc`), the tool will prompt you for the password unless it is set in your `config.yaml`.
- If you have a password in your config, it will be used automatically for decryption.

**Example:**

```cmd
dcsbr.exe restore --target D:\Temp\restore D:\Backups\backup_authentik_20250521_123456.tar.gz.enc
```

If you do not have a password in your config, you will be prompted:

```text
Enter password to decrypt backup:
```

---

### 4️⃣ Decrypt a backup file (without restoring)

To just decrypt an encrypted backup file (without extracting or restoring), use:

```cmd
dcsbr.exe decrypt --target <target-folder> <backup-archive>
```

- `<backup-archive>` must be a `.enc` file created by this tool.
- The decrypted file will be written to `<target-folder>` with the `.enc` extension removed.
- If you have a password in your config, it will be used automatically. Otherwise, you will be prompted for the password.

**Example:**

```cmd
dcsbr.exe decrypt --target D:\Temp\decrypted D:\Backups\backup_authentik_20250521_123456.tar.gz.enc
```

---

## ⚠️ Disclaimer

This tool is provided as-is, without any warranty of any kind. Use it at your own risk. The authors and contributors are not responsible for any data loss, damage, or other issues that may arise from using this software. Always test your backup and restore process before relying on it for production data.

---

## 🛠️ Features

- Multi-format backup: `tar.gz` and/or `zip`
- Backs up both stack folders and all Docker volumes
- Supports multiple source stacks
- Cross-platform (Windows, Linux, macOS)
- Simple YAML configuration
- Fast, reliable, and easy to automate

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
