# docker-compose-stack-backup-restore

Docker Compose Backup & Restore

## Usage

```bash

go test -v ./...

# go run ./cmd/dcsbr <source_folder> <destination_folder>
# go run ./cmd/dcsbr D:/Projects/docker/dnscrypt-proxy D:/Projects/docker/docker-compose-stack-backup-restore
# go run ./cmd/dcsbr .develop/authentik .develop/backup

go run ./cmd/dcsbr .develop/authentik .develop/backup

```
