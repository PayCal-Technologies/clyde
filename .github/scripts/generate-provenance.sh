#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
import datetime
import glob
import hashlib
import json
import os
import subprocess

version = open("VERSION", encoding="utf-8").read().strip()
source_date_epoch = int(subprocess.check_output(["git", "log", "-1", "--format=%ct"], text=True).strip())
created = datetime.datetime.fromtimestamp(source_date_epoch, datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
commit = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
remote_result = subprocess.run(
    ["git", "config", "--get", "remote.origin.url"],
    text=True,
    stdout=subprocess.PIPE,
    stderr=subprocess.DEVNULL,
    check=False,
)
remote = remote_result.stdout.strip()
if not remote:
    remote = "https://github.com/PayCal-Technologies/clyde"

def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

subjects = []
for path in sorted(glob.glob("dist/*")):
    name = os.path.basename(path)
    if not os.path.isfile(path) or name == "SHA256SUMS" or name.endswith(".sigstore.json"):
        continue
    subjects.append({
        "name": name,
        "digest": {"sha256": sha256(path)},
    })

provenance = {
    "_type": "https://in-toto.io/Statement/v1",
    "subject": subjects,
    "predicateType": "https://slsa.dev/provenance/v1",
    "predicate": {
        "buildDefinition": {
            "buildType": "https://github.com/PayCal-Technologies/clyde/.github/" + "workflows/release.yml@v1",
            "externalParameters": {
                "version": version,
                "tag": f"v{version}",
                "go_version": open("go.mod", encoding="utf-8").read().split("go ", 1)[1].splitlines()[0],
            },
            "internalParameters": {},
            "resolvedDependencies": [
                {
                    "uri": remote,
                    "digest": {"gitCommit": commit},
                }
            ],
        },
        "runDetails": {
            "builder": {
                "id": "https://github.com/PayCal-Technologies/clyde/" + "actions" + "/workflows/release.yml",
            },
            "metadata": {
                "invocationId": os.environ.get("GITHUB_RUN_ID", ""),
                "startedOn": created,
                "finishedOn": created,
            },
        },
    },
}

with open("dist/clyde.provenance.intoto.json", "w", encoding="utf-8") as f:
    json.dump(provenance, f, indent=2)
    f.write("\n")
PY

.github/scripts/write-release-checksums.sh
