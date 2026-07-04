# Restorable

> A backup you've never restored is a hope, not a backup.

Restorable restore-tests your backups. On schedule, the agent restores your latest
restic snapshot into a disposable sandbox, runs verification recipes against it
(file assertions, database load tests, application health checks), destroys the
sandbox, and tells you whether the restore actually worked.

**Status: pre-alpha, under active development. Not ready for use yet.**

## Layout

- `agent/` — Go agent that runs in your homelab (standalone-capable, no account required)
- `web/` — hosted control plane (Next.js): scheduling, history, alerting
- `recipes/` — declarative YAML verification recipes
- `docs/` — user documentation

See `SPEC.md` for the architecture and build plan.
