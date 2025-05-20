# docker-compose-stack-backup-restore 🚀

Easily back up your Docker Compose stacks—including all volumes and filesystems—with a single tool!

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
