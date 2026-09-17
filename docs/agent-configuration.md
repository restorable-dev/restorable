# Agent configuration

The agent reads a single YAML file (default `agent.yaml`, override with
`--config`). Minimal working config:

```yaml
repo: /srv/backups/restic
```

Full reference:

```yaml
# Required. The restic repository to test — any restic backend string:
# local path, s3:..., b2:..., sftp:..., rest:https://...
repo: /srv/backups/restic

# Name of the environment variable holding the repository password.
# Default: RESTIC_PASSWORD. The password itself never goes in this file,
# and it is never sent anywhere — it is only handed to your local restic.
password_env: RESTIC_PASSWORD

# Cron schedule for `restorable run` (daemon mode). Standard 5-field cron
# plus descriptors like "@daily". Not needed for one-shot `restorable test`.
schedule: "0 3 * * 0"

# Recipe files to run against the restored snapshot, relative to this file.
# With no recipes, a test still verifies that the restore itself succeeds.
recipes:
  - recipes/my-app.yaml

sandbox:
  # Base directory for restore sandboxes. Default: the OS temp dir.
  dir: /var/tmp/restorable
  # Extra free-space floor for the pre-flight disk check, on top of the
  # estimated snapshot size (accepts 500MB, 2GiB, ...).
  min_free_space: "1GiB"
```

## Disk-space pre-flight

Before restoring anything, the agent checks free space in the sandbox
directory. It requires the snapshot's recorded size plus 20% headroom (falling
back to 1 GiB when the snapshot predates restic 0.17 and has no size summary),
or `sandbox.min_free_space`, whichever is larger. If the space is not there,
the test aborts with a clear error and nothing is restored.

## Read-only guarantee

The agent only ever runs read operations against your repository: `version`,
`snapshots`, `ls`, `restore`, `check`, `dump`, `cat`. This whitelist is
enforced in
the code's type system — the exec wrapper physically cannot run `forget`,
`prune`, or anything else.
