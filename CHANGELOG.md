# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.6.1]

### Fixed

- Skills with non-string `metadata` values (nested lists or maps) no longer
  fail to load with a raw YAML parse error ([#92]). The spec still requires
  string keys and string values; violations now surface as the existing
  per-key frontmatter validation errors, and scoring can proceed.
- LLM judge failures with "no valid JSON object found in response" ([#91]):
  the judge model could mistake the harness's own trailing anti-injection
  reminder for injected content and respond with commentary instead of
  JSON. The judge prompts now announce the content delimiters and reminder
  as part of the harness, the reminder no longer contradicts the plain-text
  novelty follow-up, and scoring retries once with a strengthened prompt
  when a response contains no parseable JSON.

## [1.6.0]

### Added

- `--allow-nested-paths` flag for `validate structure` and `check`: allow
  deep nesting within selected skill-relative paths ([#82], thanks
  [@choplin])
- `--exclude-token-paths` flag: exclude selected subtrees from non-standard
  token accounting ([#83], thanks [@choplin])
- `-o compact` output format: one line per skill listing only warnings and
  errors ([#84], thanks [@choplin])
- Multilingual imperative-sentence detection in content analysis, with
  Chinese verb/keyword support and mixed CJK/Latin sentence splitting
  ([#79], thanks [@pinghe])

### Fixed

- False-positive "referenced without its extension" warnings: reference
  matching now requires path-word boundaries, and a same-directory sibling
  no longer matches bare words in prose ([#85], [#89])

### Security

- Hardening against malicious skill packages ([#78], thanks
  [@aminmesbahi]): expanded SSRF blocklist for link checking (CGNAT,
  multicast, IPv4-mapped IPv6, and more), refusal to read symlinks and
  other non-regular files, prompt-injection delimiters and score clamping
  for the LLM judge, `--` separator for claude CLI invocations, tightened
  score-cache permissions, bounded link-check concurrency, and escaped
  GitHub Actions annotations (also closes [#81])
- Symlink containment enforced for file reads and internal-link
  validation: paths and link targets must resolve inside the skill
  package ([#86], [#88], [#90])

## [1.5.6]

### Fixed

- Improve error message when the OpenAI API returns an `incorrect_hostname`
  error due to regional endpoint requirements ([#70]). The error now tells
  the user to set `OPENAI_BASE_URL` or `--base-url` with the correct
  regional host (e.g. `https://us.api.openai.com/v1`).

## [1.5.5]

### Fixed

- Fix false positive in comma-list keyword stuffing heuristic on prose
  descriptions with inline enumeration lists ([#71]). The heuristic now
  checks prose density (average words per segment) so that sentences with
  enough surrounding prose are not flagged as keyword dumps.
- Support `OPENAI_BASE_URL`, `OPENAI_ORG_ID`, and `OPENAI_PROJECT_ID`
  environment variables for the OpenAI LLM-as-judge provider ([#70]).
  Organization and project headers are only sent when the base URL points
  to an OpenAI endpoint.

## [1.5.4]

### Fixed

- Fix false-positive orphan warnings on Windows: file paths from the filesystem
  are now normalized to forward slashes before comparing against markdown
  references ([#63]).
- Fix code block detection on Windows: fenced code block regexes now handle
  CRLF line endings, fixing zero code-block counts when files are checked out
  with Windows-style line endings.
- Fix backslash paths in token count keys, result messages, and GitHub Actions
  annotations on Windows.
- Add Windows (`windows-latest`) to CI test matrix.

## [1.5.3]

### Fixed

- Contamination warnings now only fire when multiple application programming
  languages are detected. Skills containing only auxiliary languages (shell,
  config formats) no longer trigger false positives ([#60], [#62]).

## [1.5.2]

### Fixed

- Link checker now falls back to GET when HEAD returns 404 or 405, matching
  the standard approach used by lychee and other link validators. Fixes false
  positives on sites that don't handle HEAD correctly ([#45]).
- Link checker now sends `Accept: text/html` header, fixing false positives
  on SPAs like crates.io that require content negotiation to serve pages.

## [1.5.1]

### Fixed

- Block SSRF in link validation: the HTTP client now refuses to connect to
  private/reserved IP addresses (loopback, RFC 1918, link-local, cloud metadata
  endpoints). Each hop in a redirect chain is checked independently, preventing
  redirects to internal addresses.
- Block path traversal in internal link checks: relative links that resolve
  outside the skill directory (e.g., `../../etc/passwd`) are now rejected
  instead of being passed to `os.Stat`.

### Added

- SECURITY.md with reporting instructions and scope.
- CONTRIBUTING.md, CODE_OF_CONDUCT.md, PR template, and issue templates.

## [1.5.0]

### Added

- Add `claude-cli` LLM provider for scoring without API keys ([#43], [#44]). Uses the
  locally authenticated `claude` binary, making LLM scoring accessible to users
  with team or company subscriptions who don't have an explicit API key. Default
  model is `sonnet`.
- Preflight check for the `claude` binary at client creation time, giving a
  clear error when the CLI is not installed.

### Fixed

- Documentation notes that `claude-cli` scores may be less consistent than
  API-based providers because the CLI loads local context (CLAUDE.md, memory,
  rules) into each scoring call.

## [1.4.0]

### Added

- Add `--allow-dirs` flag to accept specific non-standard directories without
  warnings ([#39]). Allowed directories are exempt from deep-nesting checks
  and skipped for orphan detection (with an informational note). Useful for
  development directories like `evals/` or `testing/` that aren't part of the
  spec but are needed during skill development.

### Changed

- Refactor `--only` and `--skip` flags from manual comma-separated string
  parsing to `StringSliceVar`, matching the `--allow-dirs` flag style. Both
  comma-separated (`--only=structure,links`) and repeated
  (`--only=structure --only=links`) syntax are now supported. Existing
  comma-separated usage is unaffected.
- Restructure `validate structure` and `check` flag documentation in the
  README from dense prose paragraphs into scannable tables.

## [1.3.1]

### Added

- Add opt-in rate limiting for LLM API calls during evaluation via
  `RateLimit` option ([#37]). Disabled by default (zero value).
- Recognize `OWNERS.yaml` and `OWNERS` as known extraneous files so they
  produce the more specific "not needed in a skill" warning ([#33]).

### Changed

- Deduplicate regex patterns into `util/regex.go`, fixing tilde-fence
  stripping in content analysis ([#35]).
- Cache token encoder with `sync.Once` to avoid repeated initialization
  in batch runs ([#34]).

### Fixed

- Rate limiter now respects context cancellation instead of blocking
  until the next tick interval.
- First rate-limited LLM call no longer incurs an unnecessary delay.

## [1.3.0]

### Added

- Add `--allow-extra-frontmatter` flag to suppress warnings for non-spec
  frontmatter fields ([#27]). Useful for teams that embed custom metadata
  (e.g. internal tags or routing hints) alongside standard skill fields.
- Add `--allow-flat-layouts` flag to support skills that keep all files at
  the root instead of using `references/`, `scripts/`, and `assets/`
  subdirectories ([#23]). When enabled, root-level files are treated as
  standard content for token counting and orphan detection rather than
  flagged as extraneous.

### Changed

- Both new flags are available on `validate structure` and `check` commands.

## [1.2.1]

### Fixed

- Fix false positive in comma-separated keyword stuffing heuristic on
  multi-sentence descriptions with inline enumeration lists ([#26]).
  The heuristic now splits descriptions into sentences before checking,
  so commas in separate sentences are no longer counted together.

### Changed

- Extract keyword stuffing thresholds into named constants for easier tuning.

## [1.2.0]

### Changed

- Bump default OpenAI model to GPT 5.2.
- Add CI and review-skill examples to `examples/`.

## [1.1.0]

### Changed

- Increase model name truncation limit in eval compare report.

## [1.0.0]

First stable release. Includes the complete CLI and importable library packages.

### CLI

- `validate structure` — spec compliance, frontmatter, token counts, code fence
  integrity, internal link validation, orphan file detection, keyword stuffing
- `validate links` — external HTTP/HTTPS link validation with template URL support
- `analyze content` — content quality metrics (density, specificity, imperative ratio)
- `analyze contamination` — cross-language contamination detection and scoring
- `check` — run all deterministic checks with `--only`/`--skip` filtering
- `score evaluate` — LLM-as-judge scoring (Anthropic and OpenAI-compatible providers)
- `score report` — view and compare cached LLM scores across models
- Output formats: text, JSON, markdown
- GitHub Actions annotations via `--emit-annotations`
- `--strict` mode for CI (treats warnings as errors)
- Multi-skill directory detection
- Pre-commit hook support for all major agent platforms
- Homebrew install via `agent-ecosystem/tap`

### Library

- `orchestrate` — high-level validation coordination
- `evaluate` — LLM scoring orchestration with caching and progress reporting
- `judge` — LLM client abstraction and scoring (EXPERIMENTAL)
- `structure`, `content`, `contamination`, `links` — individual analysis packages
- `skill` — SKILL.md parsing (frontmatter + body)
- `skillcheck` — skill detection and reference file analysis
- `report` — output formatting (text, JSON, markdown, GitHub annotations)
- `types` — shared data types (`Report`, `Result`, `Level`, etc.)
- `judge.LLMClient` interface for custom LLM providers

[1.6.1]: https://github.com/agent-ecosystem/skill-validator/compare/v1.6.0...v1.6.1
[1.6.0]: https://github.com/agent-ecosystem/skill-validator/compare/v1.5.6...v1.6.0
[1.5.6]: https://github.com/agent-ecosystem/skill-validator/compare/v1.5.5...v1.5.6
[1.5.5]: https://github.com/agent-ecosystem/skill-validator/compare/v1.5.4...v1.5.5
[1.5.4]: https://github.com/agent-ecosystem/skill-validator/compare/v1.5.3...v1.5.4
[1.5.3]: https://github.com/agent-ecosystem/skill-validator/compare/v1.5.2...v1.5.3
[1.5.2]: https://github.com/agent-ecosystem/skill-validator/compare/v1.5.1...v1.5.2
[1.5.1]: https://github.com/agent-ecosystem/skill-validator/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/agent-ecosystem/skill-validator/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/agent-ecosystem/skill-validator/compare/v1.3.1...v1.4.0
[1.3.1]: https://github.com/agent-ecosystem/skill-validator/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/agent-ecosystem/skill-validator/compare/v1.2.1...v1.3.0
[1.2.1]: https://github.com/agent-ecosystem/skill-validator/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/agent-ecosystem/skill-validator/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/agent-ecosystem/skill-validator/compare/v1.0.0...v1.1.0
[#23]: https://github.com/agent-ecosystem/skill-validator/issues/23
[#26]: https://github.com/agent-ecosystem/skill-validator/issues/26
[#27]: https://github.com/agent-ecosystem/skill-validator/issues/27
[#33]: https://github.com/agent-ecosystem/skill-validator/issues/33
[#34]: https://github.com/agent-ecosystem/skill-validator/pull/34
[#35]: https://github.com/agent-ecosystem/skill-validator/pull/35
[#37]: https://github.com/agent-ecosystem/skill-validator/pull/37
[#39]: https://github.com/agent-ecosystem/skill-validator/issues/39
[#43]: https://github.com/agent-ecosystem/skill-validator/issues/43
[#44]: https://github.com/agent-ecosystem/skill-validator/pull/44
[#45]: https://github.com/agent-ecosystem/skill-validator/issues/45
[#60]: https://github.com/agent-ecosystem/skill-validator/issues/60
[#62]: https://github.com/agent-ecosystem/skill-validator/pull/62
[#63]: https://github.com/agent-ecosystem/skill-validator/issues/63
[#70]: https://github.com/agent-ecosystem/skill-validator/issues/70
[#71]: https://github.com/agent-ecosystem/skill-validator/issues/71
[#78]: https://github.com/agent-ecosystem/skill-validator/pull/78
[#79]: https://github.com/agent-ecosystem/skill-validator/pull/79
[#81]: https://github.com/agent-ecosystem/skill-validator/issues/81
[#82]: https://github.com/agent-ecosystem/skill-validator/pull/82
[#83]: https://github.com/agent-ecosystem/skill-validator/pull/83
[#84]: https://github.com/agent-ecosystem/skill-validator/pull/84
[#85]: https://github.com/agent-ecosystem/skill-validator/issues/85
[#86]: https://github.com/agent-ecosystem/skill-validator/issues/86
[#88]: https://github.com/agent-ecosystem/skill-validator/issues/88
[#89]: https://github.com/agent-ecosystem/skill-validator/pull/89
[#90]: https://github.com/agent-ecosystem/skill-validator/pull/90
[#91]: https://github.com/agent-ecosystem/skill-validator/issues/91
[#92]: https://github.com/agent-ecosystem/skill-validator/issues/92
[@aminmesbahi]: https://github.com/aminmesbahi
[@choplin]: https://github.com/choplin
[@pinghe]: https://github.com/pinghe
