# Docs

User-facing documentation for the Restorable agent.

- [Quickstart](quickstart.md): install → first green test in under 10 minutes
- [Agent configuration](agent-configuration.md): `agent.yaml` reference,
  disk-space pre-flight, read-only guarantee
- [Commands](commands.md): `restorable test`, `restorable run`, exit codes
- [The `files` recipe check](recipes-files.md): asserting restored content
- [Database checks](recipes-databases.md): `postgres`, `mysql`, `sqlite`
- [The `docker-app` check](recipes-docker-app.md): boot the real app on
  restored data
- [Notifications](notifications.md): ntfy, webhooks, and a healthcheck
  ping that catches a dead agent
