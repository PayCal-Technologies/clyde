#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
import datetime
import glob
import hashlib
import json
import os
import re
import subprocess

version = open("VERSION", encoding="utf-8").read().strip()
source_date_epoch = int(subprocess.check_output(["git", "log", "-1", "--format=%ct"], text=True).strip())
created = datetime.datetime.fromtimestamp(source_date_epoch, datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()

def spdx_id(prefix, value):
    return "SPDXRef-" + prefix + "-" + re.sub(r"[^A-Za-z0-9.-]", "-", value)

go_version = subprocess.check_output(["go", "version"], text=True).strip()
artifacts = sorted(
    path for path in glob.glob("dist/*")
    if os.path.isfile(path) and not path.endswith(".spdx.json") and os.path.basename(path) != "SHA256SUMS"
)

files = []
relationships = [
    {
        "spdxElementId": "SPDXRef-DOCUMENT",
        "relationshipType": "DESCRIBES",
        "relatedSpdxElement": "SPDXRef-Package-clyde",
    }
]
for path in artifacts:
    name = os.path.basename(path)
    file_id = spdx_id("File", name)
    files.append({
        "fileName": "./dist/" + name,
        "SPDXID": file_id,
        "checksums": [{"algorithm": "SHA256", "checksumValue": sha256(path)}],
        "licenseConcluded": "NOASSERTION",
        "copyrightText": "NOASSERTION",
    })
    relationships.append({
        "spdxElementId": "SPDXRef-Package-clyde",
        "relationshipType": "CONTAINS",
        "relatedSpdxElement": file_id,
    })

doc = {
    "spdxVersion": "SPDX-2.3",
    "dataLicense": "CC0-1.0",
    "SPDXID": "SPDXRef-DOCUMENT",
    "name": f"clyde-{version}",
    "documentNamespace": f"https://github.com/PayCal-Technologies/clyde/releases/tag/v{version}#spdx",
    "creationInfo": {
        "created": created,
        "creators": ["Tool: Clyde release workflow"],
    },
    "packages": [
        {
            "name": "clyde",
            "SPDXID": "SPDXRef-Package-clyde",
            "versionInfo": version,
            "downloadLocation": "https://github.com/PayCal-Technologies/clyde",
            "filesAnalyzed": True,
            "licenseDeclared": "0BSD",
            "copyrightText": "NOASSERTION",
        },
        {
            "name": "Go toolchain",
            "SPDXID": "SPDXRef-Package-go-toolchain",
            "versionInfo": go_version,
            "downloadLocation": "https://go.dev/dl/",
            "filesAnalyzed": False,
            "licenseDeclared": "BSD-3-Clause",
            "copyrightText": "NOASSERTION",
        },
        {
            "name": "notebooklm-mcp",
            "SPDXID": "SPDXRef-Package-notebooklm-mcp",
            "versionInfo": "2.0.0",
            "downloadLocation": "https://www.npmjs.com/package/notebooklm-mcp",
            "filesAnalyzed": False,
            "licenseDeclared": "NOASSERTION",
            "copyrightText": "NOASSERTION",
        },
    ],
    "files": files,
    "relationships": relationships + [
        {
            "spdxElementId": "SPDXRef-Package-go-toolchain",
            "relationshipType": "BUILD_TOOL_OF",
            "relatedSpdxElement": "SPDXRef-Package-clyde",
        },
        {
            "spdxElementId": "SPDXRef-Package-notebooklm-mcp",
            "relationshipType": "OPTIONAL_DEPENDENCY_OF",
            "relatedSpdxElement": "SPDXRef-Package-clyde",
        },
    ],
}

with open("dist/clyde.spdx.json", "w", encoding="utf-8") as f:
    json.dump(doc, f, indent=2)
    f.write("\n")
PY

.github/scripts/write-release-checksums.sh
