# Restorable

> A backup you've never restored is a hope, not a backup.

![Restorable restoring a Vaultwarden backup: a good snapshot passes, then a half-copied database that restic happily backed up fails the check](docs/assets/demo.gif)

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
  restore:   4.1s
  checks:
    ✓ nextcloud/files (3922ms): 2 required path(s) present, 5 sampled checksum(s) match repository
    ✓ nextcloud/sqlite (18ms): "data/owncloud.db" passes integrity_check
```

Works fully standalone — single binary, no account, cron-friendly exit codes.
Because every test is a real restore, you also get your **measured recovery
time** — how long being back online actually takes, not a guess. Optional
notifications push failures to your phone and catch the agent itself going
quiet.

Free and open source (MIT). There's no hosted service and no account: the
hosted dashboard from v0.1 was retired in v0.2.0, and its alerting and
dead-agent detection now run from the agent itself.

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
  `dump`, `cat`. The whitelist is enforced in the type system; `forget`/`prune`
  are a compile error, not a code-review promise.
- **Sandboxes are always destroyed** — on success, failure, and panic. Disk
  space is checked *before* any restore begins.
- **Your data never leaves your machine.** With no `notify:` block the agent
  talks to nothing but your repository. With one, it sends a redacted run
  summary only to the URLs you configured. Never file contents, never
  passwords, and it listens on no ports.

## Notifications (optional)

```yaml
notify:
  ntfy: https://ntfy.sh/<long-random-topic>   # push on failure
  webhook: https://example.com/hook           # JSON summary on failure
  healthcheck: https://hc-ping.com/<uuid>     # pinged every run
```

The healthcheck ping is the one to set up first. Failure alerts can't tell
you that tests stopped running at all; a dead-man's-switch monitor like
[Healthchecks.io](https://healthchecks.io) can. See
[notifications](docs/notifications.md).

## Docs

[Quickstart](docs/quickstart.md) ·
[Configuration](docs/agent-configuration.md) ·
[Commands](docs/commands.md) ·
[files](docs/recipes-files.md) ·
[databases](docs/recipes-databases.md) ·
[docker-app](docs/recipes-docker-app.md) ·
[Notifications](docs/notifications.md) ·
[Security](SECURITY.md)

## Building from source

```sh
git clone https://github.com/restorable-dev/restorable && cd restorable/agent
go build -o restorable .
```

Repo layout: [`/agent`](agent/) Go agent · [`/recipes`](recipes/) shipped
recipes · [`/docs`](docs/) documentation. The retired dashboard (`web/`) is in
git history before v0.2.0.

## License

[MIT](LICENSE). The agent is and stays open source — it's the point.
