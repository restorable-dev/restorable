# The `files` recipe check

The `files` check verifies that a restore actually produced the files your
application needs: required paths exist, directories contain at least a
minimum number of files, and a random sample of restored files is compared
byte-for-byte against the repository.

```yaml
name: my-app
checks:
  - type: files
    require:
      - path: config/config.php        # this file must exist
      - path: data/                    # this dir must exist...
        min_files: 100                 # ...and hold ≥ 100 files (recursive)
    checksum_sample: 5                 # verify 5 random files against the repo
```

## Fields

| Field | Meaning |
|---|---|
| `require[].path` | Path that must exist after restore, relative to the backed-up directory (the snapshot root). Absolute paths and `..` are rejected. |
| `require[].min_files` | When set on a directory, the minimum number of regular files it must contain, counted recursively. |
| `checksum_sample` | Number of randomly chosen restored files to hash (SHA-256) and compare against the same file streamed directly out of the repository with `restic dump`. `0` (default) disables sampling. |

## How paths resolve

If your backup was created with `restic backup /srv/nextcloud`, then
`path: config/config.php` refers to `/srv/nextcloud/config/config.php`.
When a snapshot contains several top-level paths, each is tried in order.

## Outcomes

- **pass** — every required path exists, counts hold, sampled checksums match.
- **fail** — an assertion did not hold (missing path, too few files, checksum
  mismatch). Exit code 1 from `restorable test`.
- **error** — the check could not run (I/O failure, repository unreadable).
  Exit code 2.
