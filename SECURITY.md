# Security

Restorable runs against your backup repository and your restored data. That
puts it in a position of trust, so this document states what it guarantees,
what it does not, and how to report a problem.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting:
[**Report a vulnerability**](https://github.com/restorable-dev/restorable/security/advisories/new).

That opens a private advisory visible only to the maintainers. Please do not
open a public issue for anything exploitable.

You can expect an acknowledgement within a week. This is a small project, so
a fix may take longer than that, and the advisory will say where things stand
rather than going quiet.

If you would rather not use GitHub, open a public issue asking for a contact
address and leave out the details.

## Supported versions

| Version | Supported |
| ------- | --------- |
| 0.1.x   | Yes       |
| < 0.1.1 | No        |

Releases before v0.1.1 were built with a Go toolchain carrying known standard
library vulnerabilities and have been removed from the releases page. There is
no auto-update mechanism by design, so upgrading is a manual reinstall.

## What the agent guarantees

These are enforced in code, not by convention, and each has tests:

- **Read-only against your repository.** The restic wrapper exposes a closed,
  unexported subcommand type whose only values are `version`, `snapshots`,
  `ls`, `restore`, `check`, `dump` and `cat`. There is no code path that
  reaches `forget`, `prune` or `backup`; it will not compile.
- **Sandboxes are destroyed.** Teardown is deferred, so it runs on every path
  out of a verification run, including a panic in recipe code. Containers are
  force-removed along with their volumes on the same guarantee.
- **Disk pre-flight.** Free space is checked before a restore begins, so an
  oversized snapshot fails with a clear error instead of filling the disk.
- **Backup contents stay on your machine.** When connected to the hosted
  control plane, only pass/fail status, timings, snapshot IDs, recipe names
  and error strings are transmitted. File contents are never uploaded.
- **Credentials and paths are scrubbed.** Strings are passed through a scrub
  and redaction step before they are logged or transmitted, including error
  messages from cleanup failures. Repository URLs containing credentials are
  stripped before they can enter a report.
- **Outbound only.** The agent polls over HTTPS. It opens no inbound ports.

Cloud connectivity is optional. `restorable test --config agent.yaml` runs the
full local loop with no account.

## Known advisories

`govulncheck` currently reports two advisories against the agent, both through
the Docker client library:

| Advisory | CVE | Severity |
| --- | --- | --- |
| GO-2026-4887 | CVE-2026-34040 | High |
| GO-2026-4883 | CVE-2026-33997 | Medium |

**Assessment: not exploitable through this agent, and not currently fixable.**

Both describe daemon-side behavior, an AuthZ plugin bypass on oversized request
bodies and an off-by-one in plugin privilege validation. The agent is a client.
It creates, execs into, and removes containers over the local Docker socket. It
does not install plugins and does not use an AuthZ plugin, so it does not drive
the affected code paths. Your exposure to these is a function of your Docker
daemon version, not of this agent; update Docker to address them.

They are also not fixable here today. The patches exist only in
`github.com/moby/moby/v2`, which has published nothing but beta releases. Moving
this project onto a beta dependency to silence two advisories it does not
exercise would trade a real risk for a cosmetic one.

**Revisit trigger:** when `github.com/moby/moby/v2` publishes a stable release,
the agent migrates to it and this section goes away.

Development dependencies of the web control plane may carry their own
advisories. Those are not shipped to users; `npm audit --omit=dev` on the
production tree is the number that matters, and it is expected to be zero.

## Out of scope

- Vulnerabilities in restic itself. Report those to
  [restic](https://github.com/restic/restic).
- Vulnerabilities in the Docker daemon, including the two above.
- Anything requiring an attacker who already has write access to your agent
  configuration or recipe files. Those are trusted input, equivalent to
  local code execution.
- Results from automated scanners without an accompanying explanation of how
  the issue is reachable in this codebase.
