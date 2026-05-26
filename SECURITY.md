# git-lrc Security

This document answers security and procurement questions for git-lrc in concrete terms.

## Quick Answers

- Vulnerability reports: use GitHub private security reporting first; use shrijith@hexmos.com if GitHub reporting is not possible.
- Response time: acknowledgement within 2 business days; for internally confirmed findings, triage and remediation work starts within 7 calendar days.
- Default behavior: git-lrc runs locally as a CLI.
- Code leaves the machine only when you submit a review or run setup/update operations that call remote APIs.
- Review submission sends staged diff bundle data to the configured LiveReview API.
- Local files include configuration at ~/.lrc.toml and repository-local state under $(git rev-parse --git-dir)/lrc.
- Security scans: gitleaks, OSV scanner, govulncheck, and Semgrep run in GitHub Actions.
- SBOM is generated and attached to release assets.

## Security Contact And Response Times

Primary private reporting channel: GitHub Security Advisories for this repository.

- Create private report: [git-lrc private vulnerability report](https://github.com/HexmosTech/git-lrc/security/advisories/new)

Fallback private channel (if GitHub reporting is unavailable): shrijith@hexmos.com.

We treat security issues as high-priority work. This address is the founder's direct inbox so reports receive immediate attention.

Disclosure process:

1. We acknowledge receipt within 2 business days.
2. We begin triage and remediation planning within 7 calendar days for findings confirmed by our internal security review.
3. We coordinate disclosure timing with the reporter for high-impact issues.

Please include reproduction steps, lrc version, platform, and impact.

## Deployment And Runtime Model

git-lrc is a local CLI that runs on developer machines and CI systems.

Runtime modes:

- Local-only file and git operations (diff generation, hook management, local state handling).
- Remote API operations when review/setup/update commands are executed.

Configured API endpoint examples:

- Local/self-hosted LiveReview API endpoint (for example localhost deployment).
- Cloud LiveReview endpoint when users choose cloud-hosted operation.

## Data Sent And Data Stored

### What git-lrc Sends Over Network

| Event | Data Sent | Destination | When It Happens |
| --- | --- | --- | --- |
| Review submission | Encoded staged diff bundle and review metadata | LiveReview API /api/v1/diff-review | When review command submits a review |
| Review polling | Review identifier and status polling requests | LiveReview API review status endpoint | Until completion or timeout |
| Setup/auth operations | Auth and setup payloads for provisioning and API key flows | LiveReview setup/auth endpoints | During setup and re-auth flows |
| Optional usage telemetry | CLI usage event payload | LiveReview usage endpoint | After review completion |
| Self-update checks/downloads | Manifest metadata and binary download request | Release/update hosting endpoints | When self-update command runs |

### What git-lrc Stores Locally

| Data Type | Storage | Why |
| --- | --- | --- |
| API key and connector state | ~/.lrc.toml | CLI authentication and connector configuration |
| Hook metadata and repo state | $(git rev-parse --git-dir)/lrc/* and managed hook paths | Hook install/uninstall and local review state |
| Review session/attestation metadata | Local SQLite and local files | Local review traceability and workflow support |
| Update lock and pending update state | Local update state files | Safe update staging and install flow |

### Data Retention

- Local files are controlled by the user or organization running the CLI.
- The CLI does not keep a separate long-term remote copy of submitted payloads after submission.
- Server-side retention and model-training policy are controlled by the LiveReview server deployment and its policy settings.

## AI Risks And Mitigations

### AI Request Guardrails In CLI (Before Data Leaves Machine)

| Risk | Automatic Handling In git-lrc | Where Implemented |
| --- | --- | --- |
| Sending too much code context | CLI sends selected diff scope instead of full repository by default | [internal/appcore/review_runtime.go](internal/appcore/review_runtime.go), [docs/LRC_README.md](docs/LRC_README.md) |
| Unsafe payload handling | Diff is wrapped in zip and encoded for transport in fixed request shape | [internal/reviewapi/helpers.go](internal/reviewapi/helpers.go), [internal/reviewmodel/types.go](internal/reviewmodel/types.go) |
| Oversized request behavior | Size-limit paths and 413 handling are enforced in review flow | [internal/appcore/review_runtime.go](internal/appcore/review_runtime.go) |
| Redirect-based credential leakage | HTTP redirect policy restricts cross-host redirect following | [network/http_client.go](network/http_client.go) |
| Insecure update transport | Self-update network paths enforce HTTPS endpoints | [network/selfupdate_operations.go](network/selfupdate_operations.go) |
| Secret disclosure in setup errors | Connector/setup error bodies redact submitted key material | [setup/connectors.go](setup/connectors.go), [setup/connectors_test.go](setup/connectors_test.go) |
| Local credential file exposure | Config file writes use restricted permission handling and atomic write paths | [storage/files.go](storage/files.go), [storage/files_test.go](storage/files_test.go) |

### AI Response Guardrails (Where They Run)

Deep model input/output sanitization is implemented in the LiveReview service. git-lrc transports review payloads and renders results, while LiveReview applies preflight and postflight sanitization for model ingress and egress.

Service-side references:

- [../LiveReview/internal/aisanitize/sanitizer.go](../LiveReview/internal/aisanitize/sanitizer.go)
- [../LiveReview/internal/aisanitize/markdown.go](../LiveReview/internal/aisanitize/markdown.go)
- [../LiveReview/internal/aiconnectors/connector.go](../LiveReview/internal/aiconnectors/connector.go)
- [../LiveReview/docs/security/llm_output_sanitization.md](../LiveReview/docs/security/llm_output_sanitization.md)

### Prompt Injection Through Code/Comments

Risk: malicious comments or diff content can influence AI output.

Current handling:

- CLI minimizes what is sent by default (selected diff scope only).
- Teams can point git-lrc to self-hosted LiveReview deployments to keep inference and policy enforcement in their own infrastructure.
- LiveReview applies input and output sanitization automatically before and after model calls.

### Insecure Suggestions

Risk: model output can contain insecure recommendations.

Current handling:

- Output remains advisory and requires human review before merge.
- Teams can enforce CI and branch protection policy in their VCS.

### AI Guardrail Verification

Relevant verification references:

- [setup/connectors_test.go](setup/connectors_test.go)
- [storage/files_test.go](storage/files_test.go)
- [../LiveReview/internal/aisanitize/postflight_test.go](../LiveReview/internal/aisanitize/postflight_test.go)
- [../LiveReview/internal/aisanitize/markdown_test.go](../LiveReview/internal/aisanitize/markdown_test.go)
- [../LiveReview/internal/api/unified_processor_v2_post_sanitize_test.go](../LiveReview/internal/api/unified_processor_v2_post_sanitize_test.go)

## Automated Security Checks

| Workflow | Badge | What It Checks | Trigger | What It Guarantees | What It Does Not Guarantee |
| --- | --- | --- | --- | --- | --- |
| gitleaks | [![gitleaks](https://github.com/HexmosTech/git-lrc/actions/workflows/gitleaks.yml/badge.svg)](https://github.com/HexmosTech/git-lrc/actions/workflows/gitleaks.yml) | Secret pattern scanning in repository history/content | Pull request, push, manual | Detects many leaked credential patterns early | Cannot guarantee zero secret exposure or catch every custom secret format |
| osv-scanner | [![osv-scanner](https://github.com/HexmosTech/git-lrc/actions/workflows/osv-scanner.yml/badge.svg)](https://github.com/HexmosTech/git-lrc/actions/workflows/osv-scanner.yml) | Dependency vulnerability scan using OSV database | Pull request, push, manual | Detects known vulnerable dependencies in scan scope | Cannot detect unknown (0-day) vulnerabilities |
| govulncheck | [![govulncheck](https://github.com/HexmosTech/git-lrc/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/HexmosTech/git-lrc/actions/workflows/govulncheck.yml) | Go package vulnerability analysis | Pull request, push, manual | Detects known Go vulnerability matches | Cannot guarantee all runtime exploit paths are covered |
| Semgrep | [![Semgrep](https://github.com/HexmosTech/git-lrc/actions/workflows/semgrep.yml/badge.svg)](https://github.com/HexmosTech/git-lrc/actions/workflows/semgrep.yml) | Static analysis for security patterns | Pull request, push, scheduled, manual | Detects many common code-level security anti-patterns | Cannot prove absence of logic flaws or business-logic abuse |
| SBOM | [![sbom](https://github.com/HexmosTech/git-lrc/actions/workflows/sbom.yml/badge.svg)](https://github.com/HexmosTech/git-lrc/actions/workflows/sbom.yml) | Software bill of materials generation (Syft) | Release publish, push (dependency-relevant files), manual | Produces auditable component inventory for releases | Does not by itself prove component safety |

## SBOM And Dependency Transparency

- Latest releases: [git-lrc latest release](https://github.com/HexmosTech/git-lrc/releases/latest)
- SBOM workflow: [sbom.yml workflow](https://github.com/HexmosTech/git-lrc/actions/workflows/sbom.yml)

On release publication, SBOM JSON artifacts are generated and uploaded to release assets for dependency audit and procurement review.

## Security Refactor Evidence (Storage And Network Split)

git-lrc completed a large code organization refactor that separates local persistence logic and outbound HTTP logic into dedicated modules.

- Storage operation inventory: [storage/storage_status.md](storage/storage_status.md)
- Network operation inventory: [network/network_status.md](network/network_status.md)

Why this matters:

1. File and SQLite operations are reviewed in one inventory for local data-at-rest risk.
2. Outbound HTTP operations are reviewed in one inventory for data-in-transit risk.
3. Changes to storage or network behavior can be audited by checking status docs during code review.

## Known Limits

- Automated scanners reduce risk but do not guarantee absence of vulnerabilities.
- If git-lrc points to a remote LiveReview deployment, review payloads are transmitted to that deployment.
- Data retention and model-training settings are determined by the selected LiveReview server deployment.

## Supported Versions

Security fixes are prioritized on currently supported, actively maintained releases. Upgrade to the latest release to receive the most recent security improvements.

## Where To Verify

- Workflow definitions: [git-lrc workflows](https://github.com/HexmosTech/git-lrc/tree/main/.github/workflows)
- Storage operations inventory: [storage/storage_status.md](storage/storage_status.md)
- Network operations inventory: [network/network_status.md](network/network_status.md)

