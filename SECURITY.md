# Security

Restorable reads your backup repository and handles your restored data, so
here's what it does and doesn't promise, and how to tell me if something's
wrong with it.

## Reporting a vulnerability

Use [GitHub's private reporting](https://github.com/restorable-dev/restorable/security/advisories/new).
That opens an advisory only the maintainers can see. Please don't file a public
issue for anything exploitable.

I'll acknowledge within a week. Fixes may take longer, it's a small project, but
the advisory will say where things stand instead of going quiet.

If you'd rather not use GitHub, open a public issue asking for a contact address
and leave the details out.

## Supported versions

| Version | Supported |
| ------- | --------- |
| 0.1.x   | Yes       |
| < 0.1.1 | No        |

Anything before v0.1.1 was built with a Go toolchain carrying known stdlib
vulnerabilities, and those releases have been pulled. There's no auto-update by
design, so upgrading means reinstalling.

## What the agent actually enforces

Not design intentions, these are structural and have tests:

The restic wrapper takes a closed, unexported subcommand type. Its only values
are `version`, `snapshots`, `ls`, `restore`, `check`, `dump` and `cat`. There's
no path to `forget`, `prune` or `backup` because it wouldn't compile.

Sandbox teardown is deferred, so it runs on every exit from a verification run,
panics included. Containers get force-removed with their volumes on the same
guarantee.

Free space is checked before a restore starts. An oversized snapshot fails with
a clear error rather than filling your disk.

The contents of your files never leave your machine. Connected to the hosted
control plane, the agent sends pass/fail, timings, snapshot IDs, recipe and
check names, check messages, error strings, and a label for the repository.

Be aware of what those last three carry, because it is more than "metadata"
suggests. The repository label is your repo string with credentials stripped,
so it still contains the host and path, and it appears in the title of every
alert sent to whatever channels you connect. Check messages can name paths from
inside the snapshot: a checksum mismatch says which file differed. If either is
more than you want reaching a third-party chat service, run the agent
standalone and skip the cloud entirely.

Strings go through scrubbing and redaction before they're logged or
transmitted, including errors from cleanup failures. Credentials embedded in
repository URLs are stripped before anything can reach a report.

Output from database checks gets the same treatment, because a failed load
quotes the data that failed: Postgres isolates row values in DETAIL and
CONTEXT, MySQL inlines them in the error itself, and both are removed. Be
aware this part is pattern matching against known shapes rather than a
guarantee. It has been wrong twice, so assume a database engine can invent a
new way to quote your data that we have not covered yet. If a database failure
must never carry row values off the machine under any circumstances, run the
agent standalone and skip the cloud. That is the only version of this promise
that does not depend on us keeping up.

The agent polls outbound over HTTPS and opens no inbound ports.

Cloud is optional either way. `restorable test --config agent.yaml` runs the
whole local loop without an account.

## Known advisories

`govulncheck` flags two against the agent, both through the Docker client
library: GO-2026-4887 (CVE-2026-34040, high) and GO-2026-4883 (CVE-2026-33997,
medium).

Neither is exploitable here, and neither is fixable yet.

Both are daemon-side: an AuthZ plugin bypass on oversized request bodies, and an
off-by-one in plugin privilege validation. This agent is a client. It creates
containers, execs into them, and removes them over the local Docker socket. It
doesn't install plugins or use an AuthZ plugin, so it never drives the affected
paths. If these worry you, your Docker daemon version is what determines your
exposure, so update Docker.

As for fixing it here: the patches only exist in `github.com/moby/moby/v2`,
which has published nothing but betas so far. I'm not moving this onto a beta
dependency over two advisories it doesn't exercise. When moby/moby/v2 goes
stable the agent migrates and this section goes away.

Dev dependencies of the web control plane may carry their own advisories. Those
don't ship. `npm audit --omit=dev` on the production tree is the number that
matters and it should read zero.

## Out of scope

Bugs in restic itself go to [restic](https://github.com/restic/restic). Same for
the Docker daemon, including the two above.

Anything that needs an attacker who already has write access to your agent
config or recipe files. Paths inside a recipe are validated and contained, so a
recipe cannot read outside the restored snapshot, but a recipe also names the
container images its database and app checks run. Treat one you did not write
the way you would treat a docker-compose file from the same source.

Raw scanner output with no explanation of how the issue is reachable in this
codebase.
