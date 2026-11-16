# Notifications

A restore test that fails silently is worse than no test, so the agent can
tell you how each run went. Add a `notify:` block to `agent.yaml`:

```yaml
notify:
  when: failure                            # or "always"
  ntfy: https://ntfy.sh/<unguessable-topic>
  webhook: https://example.com/hooks/restorable
  healthcheck: https://hc-ping.com/<uuid>
```

Every endpoint is optional. With none set, the agent reports through stdout
and its [exit code](commands.md#exit-codes) only, which is enough if cron
already mails you on failure.

| Endpoint | When it fires | What it sends |
|---|---|---|
| `ntfy` | Failed or incomplete runs (every run with `when: always`) | A push with a title, priority `high` on failure, and a text summary |
| `webhook` | Same as ntfy | `POST` of a JSON summary |
| `healthcheck` | Every run | `/start` when a run begins, then the bare URL on pass or `/fail` on failure |

Delivery is best effort. Each request times out after 15 seconds, a failed
delivery is logged to stderr, and nothing about notifications ever changes
the run's result or exit code.

## Catching a dead agent

Alerts on failure can't tell you that tests stopped running: a crashed
daemon, a broken cron line or a machine that's off all look like silence.
The `healthcheck` ping covers that. Point it at a dead-man's-switch monitor
and set the expected period to your test schedule plus some grace:

- [Healthchecks.io](https://healthchecks.io) (free tier, or self-host it)
- Anything that speaks the same protocol: a ping URL, `/start` and `/fail`

If a ping doesn't arrive on time, the monitor alerts you through whatever
channels it supports. The `/start` signal also lets it measure how long each
restore test takes and flag one that hangs.

## ntfy

[ntfy](https://ntfy.sh) pushes to your phone with no account. Topics on the
public server are readable by anyone who knows the name, so use a long random
topic, or self-host ntfy and put it behind auth (`https://user:pass@ntfy.example.com/topic`
works; the credentials are never logged).

## Webhook payload

```json
{
  "event": "restore_test.finished",
  "status": "fail",
  "summary": "Restorable: FAIL\nrepo: /srv/backups/restic\n...",
  "repo": "/srv/backups/restic",
  "snapshot_id": "cd10c302...",
  "error": "",
  "started_at": "2026-11-16T03:00:00Z",
  "finished_at": "2026-11-16T03:00:05.5Z",
  "restore_duration_ms": 4100,
  "agent_version": "v0.2.0",
  "checks": [
    {"recipe": "app", "type": "files", "status": "fail",
     "message": "required path \"config.php\" not found in restored snapshot",
     "duration_ms": 12}
  ]
}
```

`status` is `pass`, `fail` (a check failed) or `error` (the test couldn't
run). Fields may be added in later versions; existing ones won't be renamed.

## What leaves your machine

Only what's listed above, and only to URLs you configure. File contents and
the repository password never do. Before anything is sent, the agent strips
credentials from repository URLs and removes row values that database errors
quote (Postgres `DETAIL`/`CONTEXT` lines, MySQL's inlined values). That
redaction matches known error shapes, so read [SECURITY.md](../SECURITY.md)
for its limits.

The notification URLs themselves are secrets (anyone holding one can post to
it). They are never written to logs, and `restorable init` creates
`agent.yaml` with owner-only permissions.
