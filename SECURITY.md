# Security Policy

Please report suspected security issues privately. Clyde scans repositories,
prepares local source bundles, can call local Ollama endpoints, and can upload
approved bundles to NotebookLM, so reports should include command execution,
source disclosure, prompt/data handling, and supply-chain impact when supported
by a concrete path.

Email: info@paycaltech.com

## Supported Versions

| Version | Security support |
| --- | --- |
| `main` | Supported for coordinated reports against unreleased code. |
| Latest release | Supported for fixes and release notes. |
| Older releases | Triaged, with fixes normally targeting the latest release unless immediate user risk requires otherwise. |

## Reporting

Include:

- affected version or commit;
- operating system and shell;
- reproduction steps;
- expected impact;
- any relevant logs or redacted configuration.

Do not include live secrets, private keys, access tokens, or unredacted
customer data. If a proof of concept requires sensitive material, describe the
shape of the data and wait for maintainer guidance.

## Response Targets

- Initial human acknowledgement: within 3 business days.
- Initial severity assessment: within 7 business days after enough information
  is available to reproduce or reason about the report.
- Status updates: at least every 14 days while a validated report remains open.

These are targets, not guarantees. Reports involving active exploitation,
credential exposure, or source disclosure may be prioritized immediately.

## Safe Harbour

Good-faith research is welcome when it:

- avoids privacy violations, data destruction, persistence, or service
  disruption;
- uses only accounts, repositories, systems, and data you own or are
  authorized to test;
- does not attempt extortion, lateral movement, or secret extraction beyond
  what is necessary to demonstrate impact;
- reports findings promptly and privately.

Good-faith reports following this policy will not be treated as hostile solely
because they reveal a vulnerability.
