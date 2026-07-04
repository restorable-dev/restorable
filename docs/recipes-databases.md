# Database checks: postgres, mysql, sqlite

These checks prove the databases in your backup actually load — not just that
a dump file exists.

## `postgres`

Loads a pg_dump file from the restored snapshot into a throwaway postgres
container, then asserts row counts. Needs Docker.

```yaml
- type: postgres
  dump: db/nextcloud.sql        # path relative to the snapshot root
  image: postgres:16-alpine     # optional (default shown)
  timeout: 120s                 # optional readiness timeout (default 120s)
  tables:                       # optional row-count assertions
    - name: oc_users
      min_rows: 1
```

Supported dump flavors, detected automatically:

- plain SQL (`pg_dump`, `pg_dumpall`)
- custom format (`pg_dump -Fc`, loaded with `pg_restore --exit-on-error`)
- gzipped versions of either (`.gz`, as Immich's automatic backups produce)

A dump that fails to load — truncated, corrupted, or syntactically broken —
fails the check with the loader's error message. Match `image` to your
production postgres major version: dumps load forward, not always backward.

## `mysql`

Same pattern for mysqldump files, loaded into a throwaway mysql container.

```yaml
- type: mysql
  dump: db/app-dump.sql         # plain SQL or gzipped
  image: mysql:8                # optional (default shown; mariadb:11 works too)
  timeout: 180s                 # optional (mysql initializes slowly)
  tables:
    - name: users
      min_rows: 1
```

## `sqlite`

Opens the restored database file read-only (`immutable=1` — the check cannot
write, not even a WAL file) and runs `PRAGMA integrity_check`. Runs
in-process; needs no Docker.

```yaml
- type: sqlite
  path: data/owncloud.db
```

Any unusable database — missing, not SQLite, structural corruption — fails
the check with the integrity errors in the message.

## Container hygiene

Every container these checks create is labeled, force-removed after the run
(success or failure), and its anonymous volumes are removed with it. The
database inside the container is throwaway: it never contains anything that
was not already in your backup, and its port is never published beyond
127.0.0.1 on the machine running the agent.
