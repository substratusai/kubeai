# CI security and dependency automation

## Pull request checks

The Security checks workflow runs golangci-lint using `.golangci.yml` and the
version in `.golangci-lint-version`, govulncheck for the root Go module and
`examples/private-deep-chat`, and zizmor for GitHub Actions security. These
checks run on pull requests, pushes to `main`, weekly, and on manual dispatch.
zizmor uses annotations so fork pull requests do not need a write token.
Vulnerability findings in reachable Go functions fail govulncheck.

The lint job reports new findings relative to the fixed baseline commit
`c2f7008c39f3406992d277c730bfd95f9d5597f5`. The initial full run found 139 existing
findings; this baseline prevents new lint debt without suppressing linter rules.
`make lint` continues to report the full backlog. Do not advance the baseline to
hide new findings. govulncheck scans all code and does not use this baseline.

Two aggregate jobs provide stable required-check names:

- `required-tests` waits for unit/integration tests and both e2e matrices.
- `security-checks` waits for lint, both govulncheck jobs, and zizmor.

Both fail if any dependency fails, is cancelled, or is skipped. Chart validation
is the existing `lint-test` job. Do not use path filters on these required jobs.

## CodeQL default setup

A maintainer with repository administration access must enable CodeQL in
**Settings → Advanced Security → CodeQL analysis → Set up → Default**.
Select Go and verify that the initial analysis succeeds. This is a repository
setting, so this PR does not add a CodeQL analysis workflow or enable the setting.
See [GitHub's default setup documentation](https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/configure-code-scanning/configure-code-scanning).

## OpenSSF Scorecard

Scorecard runs on `main`, weekly, and on manual dispatch. It publishes a public
score, uploads a SARIF artifact, and sends findings to the Security tab. The
README badge links to the upstream repository's results. It starts showing
workflow results after the first successful run on `main`.

OIDC and security-events write permissions are limited to its analysis job.
The workflow intentionally does not run privileged scans of pull request code.

The zizmor configuration contains exact-line exceptions for persisted Git
credentials in the four existing workflows that push commits or tags. Other
checkout steps disable credential persistence. Adding artifact uploads to those
release jobs requires a new review of credential exposure.

## Dependabot policy

Version updates run weekly. Go dependencies are grouped into Kubernetes,
OpenTelemetry, and remaining minor/patch updates. Kubernetes grouping includes
`k8s.io/*` and `sigs.k8s.io/*`; maintainers still need to check their compatibility
matrix because grouping does not guarantee identical or compatible versions.
Docker images and Actions are grouped separately. Both Helm charts, both Go
modules, all five Dockerfiles, and all three Python requirements files are covered.

Go versions have a three-day cooldown, extended to fourteen days for major
updates. The shorter Go cooldown is an explicit exception to zizmor's seven-day
recommendation. Other ecosystems use seven days. Cooldowns affect version
updates, not security updates. Groups also apply to version updates by default.

## Enable Dependabot auto-merge

The workflow is opt-in and stays inactive until the following setup is complete:

1. Enable **Allow auto-merge** and squash merging in repository settings.
2. Create an **active** ruleset for `main` requiring `required-tests`, `lint-test`,
   and `security-checks`. Enable **Require branches to be up to date before
   merging**. Do not give the automation App a bypass of this ruleset. These
   check names must have appeared in a workflow run before selecting them.
3. Create and install a GitHub App restricted to this repository. Grant
   **Contents: write**, **Pull requests: write**, and **Workflows: write**
   (needed when merging updates to Actions workflow files). Metadata read is
   implicit. Do not grant administration or rule bypass permissions.
4. Store its client ID as the repository variable `AUTOMERGE_APP_CLIENT_ID`.
   Store the private key as the **Dependabot secret**
   `AUTOMERGE_APP_PRIVATE_KEY`, under **Settings → Secrets and variables →
   Dependabot**. A regular Actions secret alone is unavailable to Dependabot PRs.
5. Set the repository variable `DEPENDABOT_AUTOMERGE_ENABLED` to `true`.

Only verified Dependabot PRs from this repository targeting `main` are eligible.
Only explicit minor or patch metadata is accepted; majors, mixed groups containing
a major, unknown classifications, and Kubernetes dependencies (`k8s.io/*` and
`sigs.k8s.io/*`) require manual review. The workflow checks
that auto-merge is enabled and that effective rulesets enforce all three checks
before approving or requesting auto-merge. Legacy branch protection alone does
not satisfy this workflow's ruleset check. API errors fail the job without merging.
The merge request is bound to the event's head commit to reject stale runs.

The App must differ from the PR author to approve it. Its token also allows the
merge's push to trigger downstream builds, tests, and documentation publication;
merges using the standard `GITHUB_TOKEN` would not trigger those workflows.
No repository checkout or PR code execution occurs in the auto-merge job.
To disable new auto-merge requests, remove or set the opt-in variable to `false`;
cancel any already enabled auto-merges separately.

## CodeRabbit

Install the CodeRabbit GitHub App for this repository. `.coderabbit.yaml` selects
English reviews, the chill profile, no reviews for drafts, and title exclusions
for `build(deps)` and `WIP`. It enables Go, Docker, shell, Actions, YAML, secret,
and Markdown tooling. The configuration alone does not install the App.
