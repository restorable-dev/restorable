# Restorable

> A backup you've never restored is a hope, not a backup.

Your backups run every night. **Restorable proves they'd actually restore.**
On a schedule, it restores your latest restic snapshot into a disposable
sandbox, verifies the data actually works — files present, databases load,
the real app boots — then destroys the sandbox and tells you the result.

```
$ restorable test
restorable: PASS
  repo:      /srv/backups/restic
  snapshot:  cd10c302
  duration:  5.5s
  checks:
    ✓ nextcloud/files (3922ms): 2 required path(s) present, 5 sampled checksum(s) match repository
    ✓ nextcloud/sqlite (18ms): "data/owncloud.db" passes integrity_check
```

Works fully standalone — single binary, no account, cron-friendly exit codes.
An optional cloud dashboard adds scheduling history, stale-backup detection,
and alerts (Telegram, Discord, ntfy, email).

## Why

Everyone runs backups. Almost nobody restore-tests them — so nobody finds the
corrupted repository, the config file that fell out of the backup set, or the
database dump that stopped being written, until the day they need it back.
Restorable turns that catastrophic future discovery into a mundane
notification.

## Quickstart (~5 minutes)

Requirements: Linux or macOS, [restic](https://restic.net), and an existing
restic repository.

**1. Install:**

```sh
curl -fsSL https://raw.githubusercontent.com/restorable-dev/restorable/main/install.sh | sh
```

(Or grab a binary from [releases](https://github.com/restorable-dev/restorable/releases) —
static, no dependencies.)

**2. Set it up** — `restorable init` inspects your backup and writes the config for you:

```sh
export RESTIC_PASSWORD=…   # or point password_env at your own variable
restorable init --repo /srv/backups/restic
```

It recognizes SQLite databases, SQL dumps, and apps like Nextcloud,
Vaultwarden, and Immich, then writes an `agent.yaml` and a starter recipe
asserting their critical files — and offers to run the first test on the spot.
Review the recipe (it's yours to edit), and you're done.

Prefer to hand-write it? A recipe is just declarative YAML:

```yaml
name: my-app
checks:
  - type: files
    require:
      - path: config/config.php   # this file must restore
      - path: data/
        min_files: 100            # this dir must hold ≥100 files
    checksum_sample: 5            # verify 5 restored files byte-for-byte
```

**3. Test:**

```sh
restorable test
```

Exit codes: `0` verified · `1` verification failed · `2` could not test.
Put `restorable test` in cron, or run `restorable run` as a daemon with a
schedule in the config. That's it.

## Check types

| Check | Proves |
|---|---|
| `files` | Required paths restored; minimum file counts; sampled files hash identically to repository content |
| `sqlite` | Database passes `PRAGMA integrity_check` (in-process, read-only) |
| `postgres` | A pg_dump in the backup loads into a throwaway postgres container; row-count assertions hold |
| `mysql` | Same, for mysqldump |
| `docker-app` | The real app image boots against restored data and answers HTTP health checks |

Ready-made recipes for [Nextcloud](recipes/nextcloud.yaml),
[Vaultwarden](recipes/vaultwarden.yaml), and [Immich](recipes/immich.yaml)
ship in [`/recipes`](recipes/). Database and app checks need Docker; `files`
and `sqlite` don't.

## The guarantees

- **Read-only against your repository.** The agent shells out to *your*
  restic and can only run `version`, `snapshots`, `ls`, `restore`, `check`,
  `dump` — the whitelist is enforced in the type system; `forget`/`prune`
  are a compile error, not a code-review promise.
- **Sandboxes are always destroyed** — on success, failure, and panic. Disk
  space is checked *before* any restore begins.
- **Your data never leaves your machine.** Standalone mode talks to nobody.
  Cloud mode reports only pass/fail metadata: a repository fingerprint
  (SHA-256 of the credential-scrubbed location), snapshot IDs, check
  results, timings. Never file contents, never passwords. The agent is
  pull-based over HTTPS and listens on no ports.

## Cloud dashboard (optional)

The hosted dashboard adds run history across repos, alerting on failure,
stale-backup detection (fires when your *testing* silently dies — the
meta-failure), and monthly confidence reports. Free tier: 1 repo, monthly
tests, 1 alert channel. Pro ($8/mo or $80/yr): unlimited. The agent works
forever without it.

## Docs

[Configuration](docs/agent-configuration.md) ·
[Commands](docs/commands.md) ·
[files](docs/recipes-files.md) ·
[databases](docs/recipes-databases.md) ·
[docker-app](docs/recipes-docker-app.md) ·
[Alerts](docs/alerts.md) ·
[Cloud](docs/cloud.md)

## Building from source

```sh
git clone https://github.com/restorable-dev/restorable && cd restorable/agent
go build -o restorable .
```

Repo layout: [`/agent`](agent/) Go agent · [`/web`](web/) control plane
(Next.js + Supabase) · [`/recipes`](recipes/) shipped recipes ·
[`/docs`](docs/) documentation.

## License

[MIT](LICENSE). The agent is and stays open source — it's the point.
