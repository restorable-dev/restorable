# Docs

User-facing documentation source. Pages are added in the same phase as the
feature they document (see CLAUDE.md engineering conventions).

- [Agent configuration](agent-configuration.md) — `agent.yaml` reference,
  disk-space pre-flight, read-only guarantee
- [Commands](commands.md) — `restorable test`, `restorable run`, exit codes
- [The `files` recipe check](recipes-files.md) — asserting restored content
- [Database checks](recipes-databases.md) — `postgres`, `mysql`, `sqlite`
- [The `docker-app` check](recipes-docker-app.md) — boot the real app on
  restored data
- [Cloud dashboard](cloud.md) — registering agents, what data leaves your
  machine (and what never does)
