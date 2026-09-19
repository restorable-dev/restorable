# Quickstart — first green test in under 10 minutes

You need: a Linux or macOS machine that can reach your restic repository,
[restic](https://restic.net) installed, and the repository password.

## 1. Install the agent (1 min)

```sh
curl -fsSL https://raw.githubusercontent.com/restorable-dev/restorable/main/install.sh | sh
```

Static binary, no dependencies, no root needed (falls back to `~/.local/bin`).
Verify: `restorable version`.

## 2. Let it set itself up (30 seconds)

The fastest path — `restorable init` connects to your repository, looks at
what the latest snapshot contains, recognizes common apps, and writes a
working `agent.yaml` and starter recipe for you:

```sh
export RESTIC_PASSWORD=your-repo-password
restorable init --repo /srv/backups/restic
```

It detects SQLite databases, SQL dumps, and apps like Nextcloud, Vaultwarden,
and Immich, asserts their critical files, and offers to run the first test
immediately. Review the generated recipe, tune it to taste, done. If that
covers you, **skip to step 4**.

Prefer to write it by hand? Continue below.

## 2b. Or configure it manually (3 min)

Create `agent.yaml`:

```yaml
repo: /srv/backups/restic     # your restic repository — local path, s3:…, b2:…, sftp:…
recipes: [my-app.yaml]
```

Create `my-app.yaml` with assertions about what a good restore of *your*
backup must contain. Start small — you can grow it later:

```yaml
name: my-app
checks:
  - type: files
    require:
      - path: config/config.php    # a file that must exist, relative to what you back up
      - path: data/
        min_files: 100             # a directory that must hold at least this many files
    checksum_sample: 5             # verify 5 random restored files against the repository
```

Not sure what paths to assert? This shows what's in your latest snapshot:

```sh
restic -r /srv/backups/restic ls latest | head -50
```
 Backing up Nextcloud, Vaultwarden, or Immich? Copy a
[ready-made recipe](../recipes/) instead.

## 3. Run your first test (1 min + restore time)

```sh
export RESTIC_PASSWORD=your-repo-password    # or point password_env at your own variable
restorable test
```

You'll watch it: check disk space → restore the latest snapshot into a
temporary sandbox → run your checks → destroy the sandbox → print the verdict.

```
restorable: PASS
  duration:  12.4s
  restore:   8.1s
  checks:
    ✓ my-app/files (3922ms): 2 required path(s) present, 5 sampled checksum(s) match repository
```

**Green?** Congratulations — you now know something most people only hope:
your backup restores. **Red?** Even better — you found out today instead of
during a disaster. The message names exactly what was missing or corrupt.

The `restore` line is your measured recovery time: how long the restore
itself took, separate from verification. Watch it as your data grows — it's
the number that matters on the day you actually need it.

## 4. Make it automatic (2 min)

Cron (uses exit codes: 0 pass, 1 fail, 2 error). Results go to stdout and
progress to stderr, so keep them apart and the results file collects one
pretty-printed JSON object per run:

```
0 3 * * 0  RESTIC_PASSWORD="$(cat /etc/restorable/pw)" restorable test --config /etc/restorable/agent.yaml --json >> /var/log/restorable-results.json 2>> /var/log/restorable.log
```

Or run the built-in daemon — add `schedule: "0 3 * * 0"` to `agent.yaml` and:

```sh
restorable run
```

## 5. Optional: alerts + dashboard (3 min)

Create an account at [restorable.dev](https://restorable.dev), mint a
registration token under
**Agents**, and connect:

```sh
restorable register --url https://restorable.dev --token rrt_…
```

Every test now reports pass/fail metadata (never your data) to your
dashboard; add a Telegram/Discord/ntfy/email channel under **Alerts** and
you'll hear about failures — and about backups that quietly stop being
tested — without checking anything. See [alerts](alerts.md) and
[what leaves your machine](cloud.md).

## Troubleshooting

- `restic binary not found in PATH` — install restic; the agent wraps your
  restic, it doesn't bundle one.
- `password environment variable RESTIC_PASSWORD is not set` — export it, or
  set `password_env: MY_VAR` in agent.yaml to use a different variable.
- `insufficient disk space … aborting before restore` — the sandbox needs
  room for a full restore; point `sandbox.dir` at a bigger volume in
  agent.yaml.
- `required path "x" not found in restored snapshot` — paths are relative to
  the directory you back up: if you `restic backup /srv/app`, then
  `path: config.php` means `/srv/app/config.php`.
