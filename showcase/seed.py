"""Writes seed SQL for showcase screenshots of the retired v0.1 dashboard.

Everything here is synthetic: two agents, three repos, twelve weeks of weekly
restore tests with realistic restore-time drift, two real-shaped failures
(one resolved, one open) and one verified ntfy channel.

    python3 showcase/seed.py <user_uuid> > seed.sql
"""

import json
import random
import sys
import uuid
from datetime import datetime, timedelta, timezone

random.seed(1911)
UID = sys.argv[1]
NOW = datetime.now(timezone.utc).replace(second=0, microsecond=0)
APP = "https://restorable.dev"


def q(v):
    if v is None:
        return "null"
    if isinstance(v, (int, float)):
        return str(v)
    if isinstance(v, datetime):
        return "'" + v.isoformat() + "'::timestamptz"
    return "'" + str(v).replace("'", "''") + "'"


def hexid(n=64):
    return "".join(random.choice("0123456789abcdef") for _ in range(n))


def last_sunday_3am(weeks_back):
    d = NOW - timedelta(days=(NOW.weekday() + 1) % 7, weeks=weeks_back)
    return d.replace(hour=3, minute=0)


agents = {
    "nas": (str(uuid.uuid4()), "homelab-nas", 84, 4),
    "vps": (str(uuid.uuid4()), "offsite-vps", 61, 9),
}
repos = {
    "vault": (str(uuid.uuid4()), "/volume1/backups/vaultwarden", "nas"),
    "cloud": (str(uuid.uuid4()), "s3:s3.us-east-1.amazonaws.com/acme-restic/nextcloud", "vps"),
    "photos": (str(uuid.uuid4()), "/volume1/backups/immich", "nas"),
}

out = ["\\set ON_ERROR_STOP on", "begin;"]
for _, (aid, name, age_days, seen_min) in agents.items():
    out.append(
        "insert into public.agents (id, user_id, name, key_hash, agent_version, last_seen, created_at) values "
        f"({q(aid)}, {q(UID)}, {q(name)}, {q(hexid())}, 'v0.1.2', {q(NOW - timedelta(minutes=seen_min))}, "
        f"{q(NOW - timedelta(days=age_days))});"
    )
for _, (rid, label, _) in repos.items():
    out.append(
        "insert into public.repos (id, user_id, fingerprint, label, created_at) values "
        f"({q(rid)}, {q(UID)}, {q(hexid())}, {q(label)}, {q(NOW - timedelta(days=84))});"
    )


def check(recipe, typ, status, message, ms):
    return {"recipe": recipe, "type": typ, "status": status, "message": message, "duration_ms": ms}


runs = []  # (repo_key, run_id, started, finished, status, error, results, restore_ms)
for week in range(11, -1, -1):
    start = last_sunday_3am(week)

    # Vaultwarden: small, fast, one bad night five weeks ago.
    restore = int(3100 + (11 - week) * 60 + random.randint(-200, 200))
    if week == 5:
        res = [
            check("vaultwarden", "files", "pass", "2 required path(s) present, 5 sampled checksum(s) match repository", 41),
            check("vaultwarden", "sqlite", "fail", '"db.sqlite3" failed integrity_check: database disk image is malformed', 12),
        ]
        status = "fail"
    else:
        res = [
            check("vaultwarden", "files", "pass", "2 required path(s) present, 5 sampled checksum(s) match repository", random.randint(30, 60)),
            check("vaultwarden", "sqlite", "pass", '"db.sqlite3" passes integrity_check', random.randint(9, 20)),
        ]
        status = "pass"
    runs.append(("vault", str(uuid.uuid4()), start, start + timedelta(milliseconds=restore + 900), status, None, res, restore))

    # Nextcloud: grows every week, so the verified recovery time drifts up.
    s2 = start + timedelta(minutes=30)
    restore = int(41000 + (11 - week) * 1650 + random.randint(-900, 900))
    boot = random.randint(38000, 52000)
    res = [
        check("nextcloud", "files", "pass", "config/config.php present, 5 sampled checksum(s) match repository", random.randint(300, 600)),
        check("nextcloud", "docker-app", "pass", "nextcloud:apache booted against restored data; /status.php reports installed", boot),
    ]
    runs.append(("cloud", str(uuid.uuid4()), s2, s2 + timedelta(milliseconds=restore + boot + 2000), "pass", None, res, restore))

    # Immich: green until this week's dump came out truncated.
    s3 = start + timedelta(hours=1)
    restore = int(18500 + (11 - week) * 400 + random.randint(-600, 600))
    if week == 0:
        res = [
            check("immich", "files", "pass", "library/ and upload/ present, 5 sampled checksum(s) match repository", 820),
            check("immich", "postgres", "fail", "pg_restore failed: input file appears to be truncated (dump ends mid-COPY in table assets)", 6400),
        ]
        status = "fail"
    else:
        rows = 48210 + (11 - week) * 310
        res = [
            check("immich", "files", "pass", "library/ and upload/ present, 5 sampled checksum(s) match repository", random.randint(700, 900)),
            check("immich", "postgres", "pass", f"dump loaded; assets has {rows} rows (min 40000)", random.randint(9000, 14000)),
        ]
        status = "pass"
    runs.append(("photos", str(uuid.uuid4()), s3, s3 + timedelta(milliseconds=restore + 15000), status, None, res, restore))

for repo_key, run_id, started, finished, status, error, results, restore_ms in runs:
    rid, _, agent_key = repos[repo_key]
    out.append(
        "insert into public.test_runs (id, user_id, agent_id, repo_id, snapshot_id, status, error, results, agent_version, "
        "started_at, finished_at, restore_duration_ms, created_at) values "
        f"({q(run_id)}, {q(UID)}, {q(agents[agent_key][0])}, {q(rid)}, {q(hexid())}, {q(status)}, {q(error)}, "
        f"{q(json.dumps(results))}::jsonb, 'v0.1.2', {q(started)}, {q(finished)}, {restore_ms}, {q(finished)});"
    )

failed = [r for r in runs if r[4] == "fail"]
vault_fail = next(r for r in failed if r[0] == "vault")
photos_fail = next(r for r in failed if r[0] == "photos")
for run, resolved_after in ((vault_fail, timedelta(days=7)), (photos_fail, None)):
    repo_key, run_id, _, finished, *_ = run
    rid, label, _ = repos[repo_key]
    failing = next(c for c in run[6] if c["status"] == "fail")
    msg = f"Restore test failed — {label} — {failing['recipe']}/{failing['type']}: {failing['message']}\n{APP}/dashboard/runs/{run_id}"
    status = "resolved" if resolved_after else "firing"
    resolved_at = finished + resolved_after if resolved_after else None
    out.append(
        "insert into public.alerts (user_id, kind, subject_id, run_id, status, message, fired_at, resolved_at) values "
        f"({q(UID)}, 'run_failure', {q(rid)}, {q(run_id)}, {q(status)}, {q(msg)}, "
        f"{q(finished + timedelta(minutes=2))}, {q(resolved_at)});"
    )

out.append(
    "insert into public.alert_channels (user_id, type, config, verified, created_at) values "
    f"({q(UID)}, 'ntfy', {q(json.dumps({'topic': 'homelab-restores-8f2c'}))}::jsonb, true, {q(NOW - timedelta(days=80))});"
)
out.append("commit;")
print("\n".join(out))
print(f"-- FAIL_RUN={photos_fail[1]}", file=sys.stderr)
