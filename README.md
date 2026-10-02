# github-exporter

[![Built with Ona](https://ona.com/build-with-ona.svg)](https://app.ona.com/#https://github.com/Interested-Deving-1896/github-exporter) [![KDE Eco](https://img.shields.io/badge/KDE%20Eco-certified-brightgreen?logo=kde&logoColor=white&style=flat-square)](https://eco.kde.org/) [![Blue Angel](https://img.shields.io/badge/Blue%20Angel-DE--UZ%20215-0055a4?style=flat-square)](https://www.blauer-engel.de/en/certification/criteria)


<!-- AI:start:what-it-does -->
_Description pending._
<!-- AI:end:what-it-does -->

## Architecture

<!-- AI:start:architecture -->
_Architecture documentation pending._
<!-- AI:end:architecture -->

## Install

<!-- Add installation instructions here. This section is yours — the AI will not modify it. -->

```bash
git clone https://github.com/Interested-Deving-1896/github-exporter.git
cd github-exporter
```

## Usage

<!-- Add usage examples here. This section is yours — the AI will not modify it. -->

## Configuration


This exporter is configured via environment variables. All variables are optional unless otherwise stated. Below is a list of supported configuration values:

| Variable                      | Description                                                                        | Default                  |
|-------------------------------|------------------------------------------------------------------------------------|--------------------------|
| `ORGS`                        | Comma-separated list of GitHub organizations to monitor (e.g. `org1,org2`).        |                          |
| `REPOS`                       | Comma-separated list of repositories to monitor (e.g. `user/repo1,user/repo2`).    |                          |
| `USERS`                       | Comma-separated list of GitHub users to monitor (e.g. `user1,user2`).              |                          |
| `GITHUB_TOKEN`                | GitHub personal access token for API authentication.                               |                          |
| `GITHUB_TOKEN_FILE`           | Path to a file containing a GitHub personal access token.                          |                          |
| `GITHUB_APP`                  | Set to `true` to authenticate as a GitHub App.                                     | `false`                  |
| `GITHUB_APP_ID`               | The App ID of the GitHub App. Required if `GITHUB_APP` is `true`.                  |                          |
| `GITHUB_APP_INSTALLATION_ID`  | The Installation ID of the GitHub App. Required if `GITHUB_APP` is `true`.         |                          |
| `GITHUB_APP_KEY_PATH`         | Path to the GitHub App private key file. Required if `GITHUB_APP` is `true`.       |                          |
| `GITHUB_RATE_LIMIT_ENABLED`   | Whether to fetch GitHub API rate limit metrics (`true` or `false`).                | `true`                   |
| `GITHUB_RATE_LIMIT`           | Core API quota threshold for proactive GitHub App token refresh. When the remaining `core` requests drop below this value, a new installation token is requested. `0` disables this behaviour. | `0` |
| `GITHUB_RESULTS_PER_PAGE`     | Number of results to request per page from the GitHub API (max 100).               | `100`                    |
| `FETCH_REPO_RELEASES_ENABLED` | Whether to fetch repository release metrics (`true` or `false`).                   | `true`                   |
| `FETCH_ORGS_CONCURRENCY`      | Number of concurrent requests to make when fetching organization data.             | `1`                      |
| `FETCH_ORG_REPOS_CONCURRENCY` | Number of concurrent requests to make when fetching organization repository data.  | `1`                      |
| `FETCH_USERS_CONCURRENCY`     | Number of concurrent requests to make when fetching user data.                     | `1`                      |
| `FETCH_USERS_CONCURRENCY`     | Number of concurrent requests to make when fetching repository data.               | `1`                      |
| `API_URL`                     | GitHub API URL. You should not need to change this unless using GitHub Enterprise. | `https://api.github.com` |
| `LISTEN_PORT`                 | The port the exporter will listen on.                                              | `9171`                   |
| `METRICS_PATH`                | The HTTP path to expose Prometheus metrics.                                        | `/metrics`               |
| `LOG_LEVEL`                   | Logging level (`debug`, `info`, `warn`, `error`).                                  | `info`                   |

### Credential Precedence

When authenticating with the GitHub API, the exporter uses credentials in the following order of precedence:

1. **GitHub App credentials** (`GITHUB_APP=true` with `GITHUB_APP_ID`, `GITHUB_APP_INSTALLATION_ID`, and `GITHUB_APP_KEY_PATH`): If enabled, the exporter authenticates as a GitHub App and ignores any personal access token or token file.
2. **Token file** (`GITHUB_TOKEN_FILE`): If a token file is provided (and GitHub App is not enabled), the exporter reads the token from the specified file.
3. **Direct token** (`GITHUB_TOKEN`): If neither GitHub App nor token file is provided, the exporter uses the token supplied directly via the environment variable.

If none of these credentials are provided, the exporter will make unauthenticated requests, which are subject to very strict rate limits.

## CI

<!-- AI:start:ci -->
_CI documentation pending._
<!-- AI:end:ci -->

## Mirror chain

<!-- AI:start:mirror-chain -->
This repo is maintained in [`Interested-Deving-1896/github-exporter`](https://github.com/Interested-Deving-1896/github-exporter) and mirrored through:

```
Interested-Deving-1896/github-exporter  ──►  OpenOS-Project-OSP/github-exporter  ──►  OpenOS-Project-Ecosystem-OOC/github-exporter
```

Changes flow downstream automatically via the hourly mirror chain in
[`fork-sync-all`](https://github.com/Interested-Deving-1896/fork-sync-all).
Direct commits to OSP or OOC are detected and opened as PRs back to `Interested-Deving-1896`.
<!-- AI:end:mirror-chain -->

## Contributors

<!-- AI:start:contributors -->
_Contributors pending._
<!-- AI:end:contributors -->

## Origins

<!-- AI:start:origins -->
_Original project — no upstream influences recorded._
<!-- AI:end:origins -->

## Resources

<!-- AI:start:resources -->
_No additional resource files found._
<!-- AI:end:resources -->

## Accessibility

<!-- AI:start:accessibility -->
This repo uses automated accessibility auditing via `check-accessibility.yml`.

Checks include: CODEOWNERS ownership coverage, README screen-reader compatibility,
WCAG 2.1 AA HTML compliance, audio overview (espeak-ng), and Braille output (liblouis).




Run the [Check Accessibility](https://github.com/Interested-Deving-1896/github-exporter/actions/workflows/check-accessibility.yml)
workflow to generate the first report and accessibility artifacts.
See [DOCS/accessibility.md](https://github.com/Interested-Deving-1896/github-exporter/blob/main/DOCS/accessibility.md) for the full reference.
<!-- AI:end:accessibility -->

## License

<!-- AI:start:license -->
[MIT](https://github.com/Interested-Deving-1896/github-exporter/blob/master/LICENSE) © 2026 [Interested-Deving-1896](https://github.com/Interested-Deving-1896)
<!-- AI:end:license -->
