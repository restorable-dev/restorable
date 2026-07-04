# Commands

## `restorable test`

Runs one full restore-verification cycle and exits:

1. Loads and validates config and recipes (fails fast on typos).
2. Finds the latest snapshot in the repository.
3. Checks free disk space (see agent-configuration.md), aborts if insufficient.
4. Restores the snapshot into a disposable sandbox directory.
5. Runs every recipe check.
6. Destroys the sandbox — on every path out, including failures and panics.

```sh
restorable test --config agent.yaml          # human-readable result
restorable test --config agent.yaml --json   # machine-readable result on stdout
```

Progress goes to stderr, the result to stdout, so `--json` output is safe to
pipe. Exit codes are cron-friendly:

| Code | Meaning |
|---|---|
| 0 | Restore verified — everything passed |
| 1 | Verification failed — the restore or its contents are not what they should be |
| 2 | Could not test — configuration, repository, or infrastructure problem |

No account or network connectivity is required; this works fully offline.

## `restorable run`

Daemon mode: starts an internal cron scheduler and runs the same cycle on the
`schedule` from the config file. Results stream to stdout (one human block or,
with `--json`, one JSON document per run); logs go to stderr. If a run is
still in progress when the next tick fires, the tick is skipped and logged.
SIGINT/SIGTERM shut down gracefully, cancelling any in-flight restic call and
cleaning up the sandbox.

```sh
restorable run --config agent.yaml
```

## `restorable version`

Prints agent version, commit, build date, and platform.
