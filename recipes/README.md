# Recipes

Declarative YAML verification recipes shipped with the agent (and rendered as
docs). Each recipe is a starting point — copy it next to your `agent.yaml`
and adjust paths to how *your* backup is laid out.

- [`nextcloud.yaml`](nextcloud.yaml) — config + data files, optional
  sqlite/postgres database checks, optional full app boot
- [`vaultwarden.yaml`](vaultwarden.yaml) — vault database integrity + keys
- [`immich.yaml`](immich.yaml) — media library + database dump load
- [`forgejo.yaml`](forgejo.yaml) — default WORK_PATH and SQLite recovery checks
- [`forgejo-postgres.yaml`](forgejo-postgres.yaml) — Forgejo config and an included PostgreSQL dump
- [`forgejo-mysql.yaml`](forgejo-mysql.yaml) — Forgejo config and a MariaDB/MySQL dump with a user-row assertion

Check-type reference: see
[files](../docs/recipes-files.md),
[databases](../docs/recipes-databases.md), and
[docker-app](../docs/recipes-docker-app.md).
