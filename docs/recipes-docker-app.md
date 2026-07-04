# The `docker-app` check

The strongest verification there is: boot the application's real image
against the restored data and require the app itself to come up healthy.
Needs Docker.

```yaml
- type: docker-app
  image: nextcloud:apache
  mount:
    restored: "."               # what to mount ("." = the snapshot root)
    at: "/var/www/html"         # where the app expects its data
  env:                          # optional container environment
    SQLITE_DATABASE: nextcloud
  ready:
    http: "http://localhost:80/status.php"
    contains: '"installed":true'   # optional response-body requirement
    timeout: 180s                  # optional (default 120s)
```

## How the probe works

The port in `ready.http` is the **container** port. The agent publishes it on
an ephemeral port bound to 127.0.0.1, starts the container with the restored
data bind-mounted at `mount.at`, and polls the URL until it answers HTTP 200
(and contains `contains`, when set) or the timeout passes.

If the container exits before becoming ready, the check fails immediately
with the exit code rather than waiting out the timeout.

## Outcomes

- **pass** — the app serves HTTP 200 (with the required body) from restored data.
- **fail** — the app never became healthy: wrong/missing data, app crash,
  probe timeout. The message includes the last probe result.
- **error** — Docker unavailable or container infrastructure problems.
