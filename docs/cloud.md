# Connecting an agent to the cloud dashboard

The agent is fully useful standalone — cloud connectivity is additive. What
it adds today: run history across all your repos, stale-backup detection, and
alerting (Telegram, Discord, ntfy, email). Monthly confidence reports are
planned but not yet available.

## What leaves your machine (and what never does)

Reported per run: a repository **fingerprint** (SHA-256 of the
credential-scrubbed repo string) and scrubbed label, snapshot ID, pass/fail
status per check, check messages, timings, and the agent version.

Never transmitted: backup contents, file names from your data, repository
passwords, or credentials of any kind. Error messages are scrubbed of
embedded URL credentials before they enter a report. The agent connects
outbound over HTTPS only and listens on no ports.

## Registering

1. On the dashboard, open **Agents → Register a new agent**. This mints a
   one-time registration token (valid one hour, usable once).
2. Run the command it shows on the machine with the agent:

   ```sh
   restorable register --url https://<control-plane> --token rrt_… --name my-box
   ```

3. The agent exchanges the token for an API key, stored with owner-only
   permissions in `<user config dir>/restorable/credentials.json`
   (override with `--credentials`).

From then on, `restorable test` and `restorable run` report each result
automatically. Reporting failures never affect the local result or exit code.
Use `--no-report` to skip reporting, or delete the credentials file to
disconnect permanently.
