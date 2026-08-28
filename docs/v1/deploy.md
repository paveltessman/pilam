# Deploy

## Set up the server

Do these steps one time.

### 1. Make the deploy account

```
adduser --disabled-password --gecos "" deploy
usermod -aG docker deploy
mkdir -p /srv/pilam/backups
chown -R deploy:deploy /srv/pilam
```

### 2. Make the deploy key

Generate ssh keys:

```
ssh-keygen -t ed25519 -f ~/.ssh/pilam_deploy -C "github actions -> pilam" -N ""
```

Put the public half on the server.

Read the host key. GitHub needs it to recognise the server:

```
ssh-keyscan YOUR_VPS
```

### 3. Give the server a registry token

Make a classic personal access token with the `read:packages` scope only. Then, as `deploy`:

```
echo "THE_TOKEN" | docker login ghcr.io -u paveltessman --password-stdin
```

### 4. Add the repository secrets

CI renders `deploy/env.template` on every deploy and writes the result to `/srv/pilam/.env`. Six secrets feed it, under Settings, Secrets and variables, Actions:

| Secret               | Value                                            |
| -------------------- | ------------------------------------------------ |
| `DEPLOY_HOST`        | The server address.                              |
| `DEPLOY_USER`        | `deploy`                                         |
| `DEPLOY_SSH_KEY`     | The whole private half of `~/.ssh/pilam_deploy`. |
| `DEPLOY_KNOWN_HOSTS` | The output of `ssh-keyscan YOUR_VPS`.            |
| `SESSION_SECRET`     | `openssl rand -base64 32`                        |
| `POSTGRES_PASSWORD`  | `openssl rand -hex 24`                           |

`POSTGRES_PASSWORD` accepts any character. The renderer percent-encodes it into `DATABASE_URL`.

### 5. Add the Caddy site block

Copy the block from `deploy/Caddyfile.example` into `/etc/caddy/Caddyfile`. The DNS A record must point at this server first.

```
caddy validate --config /etc/caddy/Caddyfile
systemctl reload caddy
```

### 6. Start the stack

Merge to `main`. The workflow builds the image, renders the environment, and releases it.

### 7. Make the first account

```
docker compose -f /srv/pilam/docker-compose.prod.yml run --rm --no-deps app \
  user add --email you@example.com --name "Your Name" --root
```

### 8. Turn on the backup

As `deploy`, run `crontab -e` and add:

```
17 3 * * * /srv/pilam/backup.sh >> /srv/pilam/backups/backup.log 2>&1
```

Run `/srv/pilam/backup.sh` one time by hand first, and confirm that two files land in `/srv/pilam/backups`.

## Change a setting, or rotate a secret

Change a non-secret value in `deploy/env.template` and merge it. The next deploy carries it.

Rotate `SESSION_SECRET` by editing the repository secret and re-running the workflow. Everyone is logged out one time. Nothing else breaks.

`POSTGRES_PASSWORD` needs one extra step, because Postgres reads it only when it creates the data directory. Editing the secret alone leaves the running database on the old password, and the app then cannot connect. Change the database first:

```
docker compose -f /srv/pilam/docker-compose.prod.yml exec db \
  sh -c 'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

```
ALTER USER pilam WITH PASSWORD 'the new password';
```

Then edit the repository secret to the same value and re-run the workflow.

## Restore

Stop the app, but leave the database up:

```
docker compose -f docker-compose.prod.yml stop app
```

Restore the database. `--clean` drops each object before it recreates it:

```
docker compose -f docker-compose.prod.yml exec -T db \
  sh -c 'exec pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists' \
  < backups/pilam-20260828T031700Z.dump
```

Restore the photos:

```
docker run --rm -v pilam_media:/media -v /srv/pilam/backups:/backup:ro \
  alpine:3.22 sh -c 'rm -rf /media/* && tar -xzf /backup/media-20260828T031700Z.tar.gz -C /media'
```

Start the app:

```
docker compose -f docker-compose.prod.yml up -d
```

Restore both files from the same night. A database that names a photo the media volume does not hold gives a broken image.
