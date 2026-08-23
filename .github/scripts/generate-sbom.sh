#!/usr/bin/env bash
set -euo pipefail

version="$(cat VERSION)"
source_date_epoch="$(git log -1 --format=%ct)"
if ! created="$(date -u -d "@${source_date_epoch}" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null)"; then
  created="$(date -u -r "$source_date_epoch" +%Y-%m-%dT%H:%M:%SZ)"
fi

cat > dist/clyde.spdx.json <<EOF
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "clyde-${version}",
  "documentNamespace": "https://github.com/PayCal-Technologies/clyde/releases/tag/v${version}#spdx",
  "creationInfo": {
    "created": "${created}",
    "creators": ["Tool: Clyde release workflow"]
  },
  "packages": [
    {
      "name": "clyde",
      "SPDXID": "SPDXRef-Package-clyde",
      "versionInfo": "${version}",
      "downloadLocation": "https://github.com/PayCal-Technologies/clyde",
      "filesAnalyzed": false,
      "licenseDeclared": "0BSD",
      "copyrightText": "NOASSERTION"
    }
  ]
}
EOF
