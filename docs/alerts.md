# Alerts

Restorable alerts you when something needs attention — and tells you when it
recovers. Three conditions are watched:

| Alert | Fires when |
|---|---|
| Restore test failed | A run reports `fail` or `error` |
| Backup tests gone quiet | No successful test within the repo's staleness window |
| Agent silent | A registered agent hasn't checked in for 24 hours |

**One alert per incident.** A failing test alerts once, not on every retry.
When the condition clears — a passing run, tests resuming, the agent
returning — you get a single recovery notice and the incident closes. A new
problem after that opens a new incident.

## Staleness window

By default the window is inferred: 1.5× the gap between your two most recent
runs, with a 24-hour minimum (a repo with fewer than two runs is never
stale). Set an explicit window per repo (`stale_after`) if your cadence is
irregular. This is the meta-failure catcher: it fires when the *watcher*
dies — agent crashed, cron broken, machine off.

Daemon agents (`restorable run`) heartbeat every 6 hours, so a weekly test
schedule never looks like a dead agent. If your agent runs from system cron
instead, agent-silence alerts can be disabled per agent
(`alert_when_silent`).

## Channels

Configure on **Dashboard → Alerts**. Every channel must pass a **Test**
before it receives real alerts.

- **Telegram** — message the Restorable bot once, click *detect my chat ID*
  (or paste it), then Test.
- **Discord** — create a webhook in your server's channel settings and paste
  its URL.
- **ntfy** — pick a topic; self-hosted servers are supported (https only).
- **Email** — delivered via AWS SES.

## Cron

Stale and silent detection run from an hourly scheduled invocation of
`/api/cron/alerts` (`vercel.json`), authenticated with `CRON_SECRET`.
