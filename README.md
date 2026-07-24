# skill-validator

[![CI](https://github.com/agent-ecosystem/skill-validator/actions/workflows/ci.yml/badge.svg)](https://github.com/agent-ecosystem/skill-validator/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A CLI tool that validates and scores [Agent Skill](https://agentskills.io) packages.

Spec compliance is table stakes. `skill-validator` goes further: it checks that links actually resolve, flags files that shouldn't be in a skill directory, reports token counts so you can see how much of an agent's context window your skill will consume, analyzes content quality metrics, detects cross-language contamination, and offers LLM-as-judge scoring to evaluate skill quality across dimensions like clarity, actionability, and novelty. A spec-compliant skill that has broken links or a 60k-token reference file will technically pass the spec but perform poorly in practice.

## Table of Contents

- [Install](#install)
  - [CLI](#install-cli)
    - [Homebrew](#homebrew)
    - [Using Go](#using-go)
    - [Pre-commit hook](#pre-commit-hook)
  - [As a library](#as-a-library)
- [Command Usage](#command-usage)
  - [validate structure](#validate-structure)
  - [validate links](#validate-links)
  - [analyze content](#analyze-content)
  - [analyze contamination](#analyze-contamination)
  - [check](#check)
  - [score evaluate](#score-evaluate)
  - [score report](#score-report)
- [Output Formats](#output-formats)
  - [JSON output](#json-output)
  - [Markdown output](#markdown-output)
  - [GitHub Actions annotations](#github-actions-annotations)
- [CI Integration](#ci-integration)
  - [CI workflow example](#ci-workflow-example)
  - [Multi-skill directories](#multi-skill-directories)
- [Examples](#examples)
- [What it checks & why](#what-it-checks)
  - [Structure validation](#structure-validation-validate-structure)
    - [Flat skill layouts](#flat-skill-layouts)
    - [Allowing non-standard directories](#allowing-non-standard-directories)
    - [Allowing nesting at specific paths](#allowing-nesting-at-specific-paths)
  - [Link validation](#link-validation-validate-links)
  - [Content analysis](#content-analysis-analyze-content)
  - [Contamination analysis](#contamination-analysis-analyze-contamination)
  - [LLM scoring](#llm-scoring-score-evaluate)
- [Stability](#stability)
- [Development](#development)

## Install

### Install CLI

You can install the CLI in three ways:

- [Homebrew](#homebrew)
- [Using Go](#using-go)
- [Pre-commit hook](#pre-commit-hook)

#### Homebrew

```
brew tap agent-ecosystem/tap
brew install skill-validator
```

#### Using Go

```
go install github.com/agent-ecosystem/skill-validator/cmd/skill-validator@latest
```

Or build from source:

```
git clone https://github.com/agent-ecosystem/skill-validator.git
cd skill-validator
go build -o skill-validator ./cmd/skill-validator
```

#### Pre-commit hook

`skill-validator` supports [pre-commit](https://pre-commit.com). Platform-specific hooks are provided for all major agent platforms, so the correct skills directory is used automatically. For example, the following configuration runs the skill-validator [`check`](#check) command on the `".claude/skills/"` path:

```yaml
repos:
  - repo: https://github.com/agent-ecosystem/skill-validator
    rev: v0.5.0
    hooks:
      - id: skill-validator-claude
```

Available platform hooks: `skill-validator-amp`, `skill-validator-cline`, `skill-validator-claude`, `skill-validator-codex`, `skill-validator-copilot`, `skill-validator-cursor`, `skill-validator-gemini`, `skill-validator-goose`, `skill-validator-kiro`, `skill-validator-mistral-vibe`, `skill-validator-roo-code`, `skill-validator-trae`, `skill-validator-windsurf`.

A generic `skill-validator` hook is also available if you want to specify a custom command override and/or custom path — supply the command and path via `args`:

```yaml
hooks:
  - id: skill-validator
    args: ["check", "path/to/skills/"]
```

### As a library

The validation and scoring packages are importable for use in custom tooling, CI pipelines, and enterprise integrations:

```go
import (
    "github.com/agent-ecosystem/skill-validator/orchestrate"
    "github.com/agent-ecosystem/skill-validator/judge"
    "github.com/agent-ecosystem/skill-validator/evaluate"
)
```

API documentation and runnable examples are on [pkg.go.dev](https://pkg.go.dev/github.com/agent-ecosystem/skill-validator).

#### Custom LLM providers

The built-in clients cover Anthropic, OpenAI-compatible APIs, and the Claude CLI. For other providers, implement the `judge.LLMClient` interface:

```go
type LLMClient interface {
    Complete(ctx context.Context, systemPrompt, userContent string) (string, error)
    Provider() string
    ModelName() string
}
```

Your `Complete` method receives the scoring rubric as the system prompt and the skill/reference content as user content. Return the raw LLM response text; the judge package handles JSON parsing. `Provider()` and `ModelName()` are used for cache key generation, so they should return stable, unique values.

For OpenAI-compatible providers (Azure OpenAI, etc.), you can use the built-in client with a custom base URL:

```go
client, err := judge.NewClient(judge.ClientOptions{
    Provider: "openai",
    APIKey:   os.Getenv("AZURE_OPENAI_API_KEY"),
    BaseURL:  "https://your-resource.openai.azure.com/openai/deployments/your-deployment",
    Model:    "gpt-5.2",
})
```

For providers that need different request formats (AWS Bedrock, Google Vertex AI, local models), implement `LLMClient` directly and pass it to the scoring functions:

```go
scores, err := judge.ScoreSkill(ctx, skillContent, myClient, judge.DefaultMaxContentLen)

// Or use the evaluate package for full orchestration with caching
result, err := evaluate.EvaluateSkill(ctx, "./my-skill", myClient, evaluate.Options{
    MaxLen: judge.DefaultMaxContentLen,
})
```

## Command Usage

Commands map to skill development lifecycle stages:

| Development stage | Command | What it answers |
|---|---|---|
| Scaffolding | [`validate structure`](#validate-structure) | Does it conform to the spec and can agents use it? (structure, frontmatter, tokens, code fences, internal links, orphan files) |
| Writing content | [`analyze content`](#analyze-content) | Is the instruction quality good? (density, specificity, imperative ratio) |
| Adding examples | [`analyze contamination`](#analyze-contamination) | Am I introducing cross-language contamination? |
| Review | [`validate links`](#validate-links) | Do external links still resolve? (HTTP/HTTPS) |
| Quality scoring | [`score evaluate`](#score-evaluate) | How does an LLM judge rate this skill? (clarity, actionability, novelty, etc.) |
| Comparing models | [`score report`](#score-report) | How do scores compare across different LLM providers/models? |
| Pre-publish | [`check`](#check) | Run everything (except LLM scoring) |

Use `--version` to print the installed version.

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0` | Clean pass (no errors, no warnings) |
| `1` | Validation errors present |
| `2` | Warnings present, no errors |
| `3` | CLI/usage error (bad flags, missing args) |

Use `--strict` on `check` or `validate structure` to treat warnings as errors (exit 1 instead of 2). This is useful in CI pipelines where you want a binary pass/fail:

```
skill-validator check --strict <path>
skill-validator validate structure --strict <path>
```

For more details about how the commands are implemented and what they provide, refer to [What it Checks](#what-it-checks).

### validate structure

```
skill-validator validate structure <path>
skill-validator validate structure --skip-orphans <path>
skill-validator validate structure --strict <path>
skill-validator validate structure --allow-extra-frontmatter <path>
skill-validator validate structure --allow-flat-layouts <path>
skill-validator validate structure --allow-dirs=evals,testing <path>
skill-validator validate structure --allow-nested-paths=assets/components <path>
```

Checks spec compliance: directory structure, frontmatter fields, token limits, skill ratio, code fence integrity, internal link validity, and orphan file detection.

| Flag | Effect |
|---|---|
| `--strict` | Treat warnings as errors (exit 1 instead of 2) |
| `--skip-orphans` | Suppress warnings about unreferenced files in `scripts/`, `references/`, and `assets/` |
| `--allow-extra-frontmatter` | Suppress warnings for non-spec frontmatter fields (e.g. `user-invokable`). Standard fields are still fully validated |
| `--allow-flat-layouts` | Allow files at the skill root without warnings (see [Flat skill layouts](#flat-skill-layouts)) |
| `--allow-dirs=evals,testing` | Accept specific non-standard directories without warnings (see [Allowing non-standard directories](#allowing-non-standard-directories)) |
| `--allow-nested-paths=assets/components` | Allow deep nesting only within specific skill-relative paths (see [Allowing nesting at specific paths](#allowing-nesting-at-specific-paths)) |

```
Validating skill: my-skill/

Structure
  ✓ SKILL.md found

Frontmatter
  ✓ name: "my-skill" (valid)
  ✓ description: (54 chars)
  ✓ license: "MIT"

Markdown
  ✓ no unclosed code fences found

Tokens
  SKILL.md body:        1,250 tokens
  references/guide.md:    820 tokens
  ─────────────────────────────────────
  Total:                2,070 tokens

Result: passed
```

### validate links

```
skill-validator validate links <path>
```

Validates external (HTTP/HTTPS) links in SKILL.md. Internal (relative) links are checked by `validate structure`.

### analyze content

```
skill-validator analyze content <path>
skill-validator analyze content --per-file <path>
```

Computes content quality metrics for SKILL.md and reference markdown files:

```
Content Analysis
  Word count:               1,250
  Code block ratio:         0.32
  Imperative ratio:         0.45
  Information density:      0.39
  Instruction specificity:  0.78
  Sections: 6  |  List items: 23  |  Code blocks: 8

References Content Analysis
  Word count:               820
  ...

References Contamination Analysis
  Contamination level: low (score: 0.00)
  Scope breadth: 0
```

Metrics include word count, code block count/ratio, code languages, sentence count, imperative sentence ratio, information density, strong/weak language markers, instruction specificity, section count, and list item count. Reference files in `references/` are analyzed in aggregate. Use `--per-file` to see a breakdown by individual reference file.

### analyze contamination

```
skill-validator analyze contamination <path>
skill-validator analyze contamination --per-file <path>
```

Detects cross-language contamination — skills where code examples in one language could cause incorrect code generation in another context. Analyzes both SKILL.md and reference markdown files:

```
Contamination Analysis
  Contamination level: medium (score: 0.35)
  Primary language category: javascript
  ⚠ Language mismatch: python (1 category differ from primary)
  ℹ Multi-interface tool detected: mongodb
  Scope breadth: 4

References Contamination Analysis
  Contamination level: low (score: 0.00)
  Scope breadth: 0
```

Contamination scoring considers three factors: multi-interface tools (0.3 weight), application language mismatch across code blocks (0.4 weight), and scope breadth (0.3 weight). Auxiliary languages (shell, config formats, query languages, markup) are excluded from the mismatch calculation since they don't cause syntactic confusion with application languages. Reference files in `references/` are analyzed in aggregate. Use `--per-file` to see a breakdown by individual reference file.

### check

```
skill-validator check <path>
skill-validator check --only structure,links <path>
skill-validator check --skip contamination <path>
skill-validator check --per-file <path>
skill-validator check --skip-orphans <path>
skill-validator check --strict <path>
skill-validator check --allow-extra-frontmatter <path>
skill-validator check --allow-flat-layouts <path>
skill-validator check --allow-dirs=evals,testing <path>
skill-validator check --allow-nested-paths=assets/components <path>
```

Runs all checks (structure + links + content + contamination).

| Flag | Effect |
|---|---|
| `--only` | Comma-separated list of check groups to run (mutually exclusive with `--skip`) |
| `--skip` | Comma-separated list of check groups to skip (mutually exclusive with `--only`) |
| `--per-file` | Show per-file reference analysis alongside the aggregate |
| `--strict` | Treat warnings as errors (exit 1 instead of 2) |
| `--skip-orphans` | Suppress orphan file warnings in the structure check |
| `--allow-extra-frontmatter` | Suppress warnings for non-spec frontmatter fields |
| `--allow-flat-layouts` | Allow files at the skill root without warnings (see [Flat skill layouts](#flat-skill-layouts)) |
| `--allow-dirs=evals,testing` | Accept specific non-standard directories without warnings (see [Allowing non-standard directories](#allowing-non-standard-directories)) |
| `--allow-nested-paths=assets/components` | Allow deep nesting only within specific skill-relative paths (see [Allowing nesting at specific paths](#allowing-nesting-at-specific-paths)) |

Valid check groups: `structure`, `links`, `content`, `contamination`.

### score evaluate

Uses an LLM-as-judge approach to score skill quality across multiple dimensions. This is based on findings from the [agent-skill-analysis](https://github.com/dacharyc/agent-skill-analysis) research project, which identified **novelty** as a key predictor of skill value — skills that provide genuinely novel information are more likely to improve LLM outputs, while skills that restate common knowledge can potentially degrade performance.

```
export ANTHROPIC_API_KEY=your-key-here
skill-validator score evaluate <path>
skill-validator score evaluate --skill-only <path>
skill-validator score evaluate --refs-only <path>
skill-validator score evaluate --display files <path>
skill-validator score evaluate path/to/references/api-guide.md

# Or use the Claude CLI (no API key needed if already authenticated)
skill-validator score evaluate --provider claude-cli <path>
```

**Provider support**: Use `--provider` to select the backend:

| Provider | Env var | Default model | Covers |
|---|---|---|---|
| `anthropic` (default) | `ANTHROPIC_API_KEY` | `claude-sonnet-4-5-20250929` | Anthropic |
| `openai` | `OPENAI_API_KEY` | `gpt-5.2` | OpenAI, Ollama, Together, Groq, Azure, etc. \*\* |
| `claude-cli` | _(none)_ | `sonnet` | Claude CLI (uses locally authenticated `claude` binary) \* |

\* **Accuracy note:** The `claude-cli` provider shells out to the `claude` CLI, which loads local context (CLAUDE.md files, project memory, rules) into each scoring call. This extra context may influence scores, making them less reproducible across environments compared to the API-based providers. For the most consistent results, use the `anthropic` or `openai` providers with an API key.

\*\* The `openai` provider also reads these optional environment variables: `OPENAI_BASE_URL` (API base URL, overridden by `--base-url`), `OPENAI_ORG_ID` (sent as `OpenAI-Organization` header), and `OPENAI_PROJECT_ID` (sent as `OpenAI-Project` header). The org/project headers are only sent when targeting an OpenAI endpoint (e.g. `api.openai.com`, `us.api.openai.com`), not when using third-party compatible APIs.

If your OpenAI organization is bound to a regional endpoint (e.g. `us.api.openai.com` or `eu.api.openai.com`), set `OPENAI_BASE_URL` to the full regional URL including the `/v1` path:

```bash
export OPENAI_BASE_URL=https://us.api.openai.com/v1
```

Use `--model` to override the default model and `--base-url` to point at any OpenAI-compatible endpoint (e.g. `http://localhost:11434/v1` for Ollama). If the endpoint requires a specific token limit parameter, use `--max-tokens-style` to override auto-detection:

| Value | Behavior |
|---|---|
| `auto` (default) | Uses `max_completion_tokens` for o-series and gpt-5+ models, `max_tokens` for everything else |
| `max_tokens` | Always sends `max_tokens` (needed by some OpenAI-compatible providers like Ollama) |
| `max_completion_tokens` | Always sends `max_completion_tokens` |

```
Scoring skill: my-skill/

SKILL.md Scores
  Clarity:              4/5
  Actionability:        5/5
  Token Efficiency:     3/5
  Scope Discipline:     4/5
  Directive Precision:  4/5
  Novelty:              4/5
  ──────────────────────────────
  Overall:              4.00/5

  "Well-structured skill with clear proprietary API conventions."
  Novel details: References internal FooService API endpoints and a custom retry policy not in public documentation.

Reference Scores (2 files)
  Clarity:              4/5
  Instructional Value:  4/5
  Token Efficiency:     4/5
  Novelty:              3/5
  Skill Relevance:      5/5
  ──────────────────────────────
  Overall:              4.00/5
```

**Targeting**:
- Pass a skill directory to score everything (SKILL.md + references)
- Use `--skill-only` to score just SKILL.md, `--refs-only` for just references
- Pass a specific file path (e.g. `path/to/references/api-guide.md`) to score a single reference file — useful for iterating on one file without burning API calls on everything else

**Troubleshooting**: If you see an error like this:

```
Error scoring mongodb-schema-design: scoring SKILL.md: scoring SKILL.md: API returned status 400: {
  "error": {
    "message": "Unsupported parameter: 'max_tokens' is not supported with this model. Use 'max_completion_tokens' instead.",
    "type": "invalid_request_error",
    "param": "max_tokens",
    "code": "unsupported_parameter"
  }
}
```

The model requires a different token limit parameter than what auto-detection selected. Use `--max-tokens-style` to force the correct one. The error message tells you which parameter the model expects:

```
# Error says to use max_completion_tokens
skill-validator score evaluate --provider openai --model o3 --max-tokens-style max_completion_tokens <path>

# Error says to use max_tokens (e.g. with some OpenAI-compatible providers)
skill-validator score evaluate --provider openai --base-url http://localhost:11434/v1 --max-tokens-style max_tokens <path>
```

Auto-detection works for most OpenAI models, but OpenAI-compatible providers (Ollama, vLLM, Groq, etc.) vary in which parameter they support. When in doubt, check your provider's documentation.

**Content truncation**: By default, file content is truncated to 8,000 characters before sending to the LLM. Use `--full-content` to send the entire file — useful for large reference files where the scoring should account for all content, at the cost of higher token usage.

**Caching**: Results are cached in `.score_cache/` inside the skill directory. Cache keys are based on provider, model, and file path, so different models produce separate cache entries while editing a file and re-running overwrites the previous result for that file. Use `--rescore` to force re-scoring and overwrite cached results.

### score report

```
skill-validator score report <path>
skill-validator score report --list <path>
skill-validator score report --compare <path>
skill-validator score report --model claude-sonnet-4-5-20250929 <path>
```

Views and compares cached LLM scores without making API calls.

- **Default** (no flags): shows the most recent scores for each file
- `--list`: tabular summary of all cached entries with metadata (model, timestamp, provider)
- `--compare`: side-by-side comparison of dimension scores across different models
- `--model`: filter to scores from a specific model

The `--compare` flag is useful for understanding how different models perceive your skill's quality. For example, scoring with both Claude and GPT-4o can reveal whether novelty ratings are consistent across model families, or whether one model finds your instructions clearer than another.

## Output Formats

All commands accept `-o text` (default), `-o json`, or `-o markdown` for output format. Use `--emit-annotations` with any format to emit GitHub Actions workflow annotations alongside normal output.

### JSON output

Use `-o json` for machine-readable output:

```
skill-validator check -o json my-skill/
```

```json
{
  "skill_dir": "/path/to/my-skill",
  "passed": true,
  "errors": 0,
  "warnings": 0,
  "results": [
    { "level": "pass", "category": "Structure", "message": "SKILL.md found", "file": "SKILL.md" }
  ],
  "token_counts": {
    "files": [
      { "file": "SKILL.md body", "tokens": 1250 },
      { "file": "references/guide.md", "tokens": 820 }
    ],
    "total": 2070
  },
  "content_analysis": {
    "word_count": 1250,
    "code_block_count": 5,
    "code_block_ratio": 0.25,
    "code_languages": ["python", "bash"],
    "imperative_ratio": 0.35,
    "information_density": 0.30,
    "instruction_specificity": 0.78,
    "section_count": 4,
    "list_item_count": 12
  },
  "references_content_analysis": { "..." : "same shape as content_analysis" },
  "contamination_analysis": {
    "multi_interface_tools": ["mongodb"],
    "contamination_score": 0.35,
    "contamination_level": "medium",
    "language_mismatch": true,
    "scope_breadth": 4
  },
  "references_contamination_analysis": { "..." : "same shape as contamination_analysis" },
  "reference_reports": [
    {
      "file": "guide.md",
      "content_analysis": { "..." : "same shape" },
      "contamination_analysis": { "..." : "same shape" }
    }
  ]
}
```

The `passed` field is `true` when `errors` is `0`. Each result includes a `file` field (relative to the skill directory) and an optional `line` field when line-level context is available; both are omitted from JSON when empty. Token count, content analysis, and contamination analysis sections are omitted when not computed. The `reference_reports` array is only included with `--per-file`. Pipe to `jq` for post-processing:

```
skill-validator check -o json my-skill/ | jq '.content_analysis'
skill-validator check -o json my-skill/ | jq '.results[] | select(.level == "error")'
```

### Markdown output

Use `-o markdown` for GitHub-flavored markdown output. This is useful in CI pipelines where you want to write results directly to the GitHub Actions step summary:

```yaml
- name: Validate skills
  run: |
    skill-validator check -o markdown my-skill/ >> $GITHUB_STEP_SUMMARY
```

The markdown format renders results as headings, lists, and tables:

```markdown
## Validating skill: my-skill/

### Structure

- **Pass:** SKILL.md found

### Frontmatter

- **Pass:** name: "my-skill" (valid)
- **Pass:** description: (54 chars)

### Tokens

| File | Tokens |
| --- | ---: |
| SKILL.md body | 1,250 |
| references/guide.md | 820 |
| **Total** | **2,070** |

### Content Analysis

| Metric | Value |
| --- | ---: |
| Word count | 1,250 |
| Code block ratio | 0.32 |
| Imperative ratio | 0.45 |
| Information density | 0.39 |
| Instruction specificity | 0.78 |
| Sections | 6 |
| List items | 23 |
| Code blocks | 8 |

**Result: passed**
```

All three command groups support markdown output: `check`, `score evaluate`, and `score report`.

### GitHub Actions annotations

Use `--emit-annotations` to emit [GitHub Actions workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands) alongside normal output. Errors and warnings appear as inline annotations in the PR diff view, pinned to the relevant file:

```yaml
- name: Validate skills
  run: |
    skill-validator check --emit-annotations my-skill/
```

The flag works with any output format (`text`, `json`, `markdown`) and with both single-skill and multi-skill directories. Annotations are appended after the normal output:

```
Result: 3 errors, 1 warning

::warning title=Structure::unknown directory: extras/
::error file=my-skill/SKILL.md,title=Frontmatter::name is required
::error file=my-skill/SKILL.md,line=5,title=Markdown::unclosed code fence starting at line 5
```

File paths are relative to the working directory (the repository root in CI). Results at the pass and info levels are skipped. You can combine this with other flags:

```
skill-validator check --emit-annotations --strict -o markdown my-skill/ >> $GITHUB_STEP_SUMMARY
```

## CI Integration

### CI workflow example

Here's a complete GitHub Actions workflow that validates skills on every pull request. It uses `--strict` for a binary pass/fail, `--emit-annotations` to surface errors and warnings inline in the PR diff, and appends a markdown report to the job summary:

```yaml
name: Validate Skills
on:
  pull_request:
    paths:
      - "skills/**"

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install skill-validator
        run: |
          brew install agent-ecosystem/tap/skill-validator

      - name: Validate skills
        run: |
          skill-validator check --strict --emit-annotations skills/
          skill-validator check --strict -o markdown skills/ >> "$GITHUB_STEP_SUMMARY"
```

The first `check` line runs validation with text output and annotations; the second appends a markdown report to the GitHub Actions step summary. If you only need one of these (annotations in the diff or the summary report), drop the other line.

For in-progress skills where you want warnings to be non-blocking, remove `--strict` so warnings exit with code 2 instead of failing the job. You can make this conditional per directory:

```yaml
      - name: Validate published skills
        run: |
          skill-validator check --strict --emit-annotations skills/published/
          skill-validator check --strict -o markdown skills/published/ >> "$GITHUB_STEP_SUMMARY"

      - name: Validate draft skills
        run: |
          skill-validator check --emit-annotations skills/drafts/
          skill-validator check -o markdown skills/drafts/ >> "$GITHUB_STEP_SUMMARY"
```

### Multi-skill directories

If the given path does not contain a `SKILL.md` but has subdirectories that do, the validator automatically detects and validates each skill. This is useful when skills are organized as sibling directories (e.g. `skills/algorithmic-art/`, `skills/brand-guidelines/`). Symlinks are followed during detection.

```
skill-validator check skills/
```

Each skill is validated independently. The text output separates skills with a line and appends an overall summary. The JSON output wraps individual skill reports in a `skills` array:

```json
{
  "passed": false,
  "errors": 3,
  "warnings": 1,
  "skills": [
    { "skill_dir": "...", "passed": true, "errors": 0, "warnings": 0, "results": [...] },
    { "skill_dir": "...", "passed": false, "errors": 3, "warnings": 1, "results": [...] }
  ]
}
```

If no `SKILL.md` is found at the root or in any immediate subdirectory, the validator exits with code 3 (CLI error).

## Examples

The [`examples/`](examples/) directory contains ready-to-use workflows that extend skill-validator:

- **[review-skill](examples/review-skill/)** — An Agent Skill that walks a coding agent through a full skill review (structural validation, content checks, LLM scoring with Anthropic, OpenAI, or Claude CLI). Copy it into your agent's skill directory to iterate on skills during local development before requesting a human review.
- **[ci](examples/ci/)** — A GitHub Actions workflow and companion script that validate changed skills on every pull request. Copy into your repo's `.github/` directory to enforce a minimum quality bar before merging.

See the [examples README](examples/README.md) for setup instructions.

## What it checks

- [Structure validation](#structure-validation-validate-structure)
- [Link validation](#link-validation-validate-links)
- [Content analysis](#content-analysis-analyze-content)
- [Contamination analysis](#contamination-analysis-analyze-contamination)
- [LLM scoring](#llm-scoring-score-evaluate)

### Structure validation (`validate structure`)

These checks validate conformance with the [Agent Skills specification](https://agentskills.io/specification) and perform additional checks:

- **Structure**: `SKILL.md` exists; only recognized directories (`scripts/`, `references/`, `assets/`); no deep nesting; no orphan files
- **Frontmatter**: required fields (`name`, `description`) are present and valid; `name` is lowercase alphanumeric with hyphens (1-64 chars) and matches the directory name; optional fields (`license`, `compatibility`, `metadata`, `allowed-tools`) conform to expected types and lengths; unrecognized fields are flagged

**Extraneous file detection**
- Files like `README.md`, `CHANGELOG.md`, and `LICENSE` are flagged at the skill root -- these are for human readers, not agents, and may be loaded into the context window unnecessarily
- `AGENTS.md` gets a specific warning: it's for repo-level agent configuration, not skill content, and should live outside the skill directory
- Unknown files suggest moving content into `references/` or `assets/` as appropriate
- Unknown directories report how many files they contain and suggest standard alternatives (when applicable)
- Based on Anthropic's [skill-creator](https://github.com/anthropics/skills/blob/main/skills/skill-creator/SKILL.md): *"A skill should only contain essential files that directly support its functionality"*

> [!TIP]
> Extraneous file detection and recognized directories are based on the Agent Skills specification. Platform support for the spec may vary; some platforms show using different directory structures and additional files at skill root. Adhering to the spec is the best way to validate skill content is portable across platforms, so skill-validator checks against the spec.

**Keyword stuffing detection**
- Descriptions with 5+ quoted strings are flagged when the surrounding prose has fewer words than the number of quoted strings — a prose sentence followed by a supplementary trigger list (e.g., `Triggers: "term1", "term2"`) is fine
- Descriptions with 8+ comma-separated short segments (after excluding quoted strings) are flagged as keyword lists
- Per the spec, the description should concisely describe what the skill does and when to use it

**Token counting and limits**
- Reports per-file and total token counts (using `o200k_base` encoding)
- SKILL.md body: warns if over 5,000 tokens or 500 lines (per spec recommendation)
- Per reference file: warns at 10,000 tokens, errors at 25,000 tokens
- Total references: warns at 25,000 tokens, errors at 50,000 tokens
- Asset files: text-based files in `assets/` (`.md`, `.tex`, `.py`, `.yaml`, `.yml`, `.tsx`, `.ts`, `.jsx`, `.sty`, `.mplstyle`, `.ipynb`) are counted and reported in an "Asset files" section — these are templates, guides, and configs that LLMs load into context; non-text assets (images, binaries) are ignored
- Non-standard files (anything outside SKILL.md, references/, scripts/, assets/) are scanned separately and reported in an "Other files" section with per-file and total token counts
- Other files total: warns at 25,000 tokens, errors at 100,000 tokens

**Holistic structure check**
- If non-standard content exceeds 10x the standard structure content (and is over 25,000 tokens), the validator errors with a clear message that the directory doesn't appear to be structured as a skill

**Markdown validation**
- Checks SKILL.md and reference files for unclosed code fences (`` ``` `` or `~~~`)
- An unclosed fence causes agents to misinterpret everything after it as code
- Unclosed fences are reported as errors (not warnings) because they break agent usability

**Internal link validation**
- Relative links in SKILL.md are resolved against the skill directory and checked for existence
- A broken internal link means the skill references a file that doesn't exist in the package -- this is a structural problem, not a network issue, so it's checked here rather than in `validate links`
- Broken internal links are reported as errors

**Orphan file detection**
- Files in `scripts/`, `references/`, and `assets/` use progressive disclosure: they're only loaded when an agent encounters a reference to them. If a file is never mentioned anywhere reachable from SKILL.md, an agent has no signal to load it.
- The checker walks a reachability graph starting from the SKILL.md body using string containment (not just markdown links). If the relative path `references/guide.md` appears anywhere in a file's text, it counts as referenced. This catches markdown links, bare path mentions, inline code, and code blocks.
- Reachability is transitive: if SKILL.md references `references/guide.md`, and that file mentions `scripts/extract.py`, the script is considered reachable (reported as an indirect reference).
- Root-level files next to SKILL.md (e.g., `FORMS.md`, `package.json`) participate as intermediaries. If SKILL.md mentions `FORMS.md` and that file references scripts, those scripts are considered reachable. Root file matching is case-insensitive to handle casing differences between references and filenames.
- Directory-relative paths are resolved: if `references/guide.md` references `images/diagram.png`, the checker resolves that to `references/images/diagram.png`.
- Files referenced without their extension (e.g., `scripts/check_fields` instead of `scripts/check_fields.py`) get a specific warning identifying the source file and suggesting the author include the extension so agents can reliably locate the file.
- Python import chains are resolved: if a reached `.py` file contains `from helpers.merge_runs import merge`, the checker resolves this to `helpers/merge_runs.py` relative to the importing file's directory. Relative imports (`.module`, `..module`) are handled with correct Python package semantics. This prevents false positives from Python modules that are imported by referenced scripts but never mentioned by file path.
- `__init__.py` files are excluded from orphan checks entirely since they are Python package markers that are never referenced by name. However, they still act as bridge files for package imports: if `pack.py` does `from validators import X` and `validators/__init__.py` re-exports from `.base` and `.docx`, those sibling modules are considered reachable.
- Result levels:
  - **Pass**: all files in a directory are referenced
  - **Warning**: file is unreferenced (potential orphan) or referenced without its file extension
- Root-level files are not checked for orphan status since they already get non-standard structure warnings from the extraneous file check

**Flat skill layouts**

The spec recommends organizing supplemental files into `scripts/`, `references/`, and `assets/` directories. By default, the validator warns about files placed at the skill root alongside SKILL.md and counts them as non-standard content for token limits.

The `--allow-flat-layouts` flag relaxes these rules for skill authors who prefer a flat directory structure:

```
my-skill/
├── SKILL.md
├── guide.md
├── extract.py
└── template.txt
```

When enabled, the flag changes three behaviors:

1. **Structure warnings suppressed**: root-level files no longer trigger extraneous file warnings. Unknown directories still produce warnings since a flat layout means no directories, not non-standard directories.
2. **Token reclassification**: root-level text files are counted as standard content (subject to the same per-file and aggregate thresholds as `references/` files) instead of "other" files. This also prevents the holistic "not structured as a skill" error from triggering on flat layouts with substantial content.
3. **Orphan detection**: root-level files are checked for references in SKILL.md, just like files in recognized directories. Unreferenced root files are flagged as potential orphans.

```
skill-validator validate structure --allow-flat-layouts my-skill/
skill-validator check --allow-flat-layouts my-skill/
```

> [!NOTE]
> The standard directory structure remains the recommended approach for maximum portability across agent platforms. Use `--allow-flat-layouts` when a flat layout better fits your workflow, with the understanding that some platforms may not discover files outside the recognized directories.

**Allowing non-standard directories**

The spec defines three recognized directories (`scripts/`, `references/`, `assets/`). Any other directory at the skill root produces a warning. This relates to cross-platform skill file loading considerations described in [agent-ecosystem/agent-skill-implementation](https://github.com/agent-ecosystem/agent-skill-implementation).

Some development workflows use additional directories that may produce unexpected behavior across agent platforms. For example, the [evaluating-skills guide](https://agentskills.io/skill-creation/evaluating-skills) recommends an `evals/` directory for evaluation test cases, and teams may keep integration test fixtures in a `testing/` directory. If you are not distributing cross-platform skills and want to suppress warnings for specific directories that you know your preferred agent platform supports, use the `--allow-dirs` flag to suppress warnings for specific directories by name:

```
skill-validator validate structure --allow-dirs=evals my-skill/
skill-validator check --allow-dirs=evals,testing my-skill/
```

The flag accepts a comma-separated list or can be repeated (`--allow-dirs=evals --allow-dirs=testing`). Allowed directories differ from recognized directories in two ways:

1. **Exempt from deep-nesting checks**: The validator can't know the expected internal structure of arbitrary directories, so subdirectories like `evals/files/` won't trigger nesting warnings.
2. **Skipped for orphan detection**: Since the validator doesn't know how these directories are used, it skips orphan file checks for them and emits an informational note instead.

Directories not in the allow list still produce the standard warning with file counts and suggestions. If an allowed directory name matches a recognized directory (e.g., `--allow-dirs=scripts`), it's silently accepted with no change in behavior.

> [!NOTE]
> Allowing a directory suppresses validator warnings but does not change how agent platforms handle the directory. Files in non-standard directories may not be discovered during skill activation, or may load into agent context unexpectedly. If you're distributing skills across platforms, consider whether those files belong in `references/` or `assets/` instead.

**Allowing nesting at specific paths**

Some recognized directories intentionally contain self-contained bundles. For example, `assets/components/<name>/` may group templates and supporting files by component. Use `--allow-nested-paths` to suppress only the deep-nesting warning for an explicitly selected subtree:

```
skill-validator validate structure --allow-nested-paths=assets/components my-skill/
skill-validator check --allow-nested-paths=assets/components,assets/themes my-skill/
```

Paths are relative to the skill root. The flag accepts a comma-separated list or can be repeated, normalizes `/` and `\` separators, and rejects absolute paths or paths that escape the skill root. Matching observes path boundaries: allowing `assets/components` does not allow `assets/components-extra`.

This option affects only deep-nesting warnings. Structure checks outside the selected subtree and all frontmatter, orphan, token, Markdown, and link checks continue unchanged. When a nesting warning is suppressed, the report includes an informational result identifying the allowed path.

### Link validation (`validate links`)

- Checks external (HTTP/HTTPS) links only -- internal (relative) links are validated by `validate structure`
- HTTP/HTTPS links are verified with a HEAD request (10s timeout, concurrent checks)
- Template URLs using [RFC 6570](https://www.rfc-editor.org/rfc/rfc6570) syntax are skipped (e.g. `https://github.com/{OWNER}/{REPO}/pull/{PR}`)

> [!TIP]
> HTTP 403 responses are reported as `info` rather than errors, since many sites (e.g. doi.org, science.org, mathworks.com) block automated HEAD requests while working fine in browsers. A 403 doesn't necessarily mean the link is broken -- but it does mean the validator couldn't verify it. If your skill includes 403-flagged links, keep in mind that sites blocking the validator's requests may also block requests from LLM agents. If an agent can't access a linked resource, the link wastes context without providing value. Where possible, consider providing the content directly in `references/` rather than linking to it, or offer an alternate source that doesn't restrict automated access. If the links are for human readers rather than agent use, consider removing them from the skill entirely.

### Content analysis (`analyze content`)

Computes content quality metrics for SKILL.md and markdown files in `references/` (aggregate and per-file):

- **Word count**: total words in SKILL.md
- **Code block count / ratio**: number and proportion of fenced code blocks
- **Code languages**: language identifiers from code block markers
- **Sentence count**: approximate sentences (split on punctuation and blank lines, after stripping code)
- **Imperative count / ratio**: sentences starting with imperative verbs (use, run, create, configure, etc.)
- **Strong markers**: directive language count (must, always, never, required, ensure, etc.)
- **Weak markers**: advisory language count (may, consider, could, optional, suggested, etc.)
- **Instruction specificity**: strong / (strong + weak) — how directive vs advisory the language is
- **Information density**: (code_block_ratio * 0.5) + (imperative_ratio * 0.5)
- **Section count**: H2+ headers
- **List item count**: bullet and numbered list items

### Contamination analysis (`analyze contamination`)

Detects cross-language contamination — where code examples in one language could cause incorrect generation in another context. Analyzes SKILL.md and markdown files in `references/` (aggregate and per-file):

- **Multi-interface tools**: detects tools with many language bindings (MongoDB, AWS, Docker, Kubernetes, Redis, etc.) by scanning the skill name and content
- **Language categories**: maps code block languages to broad categories (shell, javascript, python, java, systems, config, etc.)
- **Language mismatch**: code blocks spanning different application language categories (auxiliary categories like shell, config, query, and markup are excluded)
- **Technology references**: framework/runtime mentions (Node.js, Django, Flask, Spring, Rails, etc.)
- **Scope breadth**: number of distinct technology categories referenced
- **Contamination score**: 3-factor formula — multi_interface (0.3) + application language mismatch (0.4) + breadth (0.3), capped at 1.0
- **Contamination level**: high (≥0.5), medium (≥0.2), low (<0.2)

### LLM scoring (`score evaluate`)

Uses an LLM-as-judge approach to evaluate skill content. The scoring prompts instruct the LLM to evaluate content on specific quality dimensions, returning structured JSON scores.

**SKILL.md** is scored on 6 dimensions (1-5 each):
- **Clarity**: How clear and unambiguous are the instructions?
- **Actionability**: Can an agent follow them step-by-step?
- **Token Efficiency**: Does every token earn its place in the context window?
- **Scope Discipline**: Does it stay focused on its stated purpose?
- **Directive Precision**: Does it use precise directives (must, always, never) vs vague suggestions?
- **Novelty**: How much content goes beyond what an LLM already knows from training data?

**Reference files** are scored on 5 dimensions (1-5 each):
- **Clarity**, **Token Efficiency**, **Novelty** (same as above)
- **Instructional Value**: Does it provide concrete, directly-applicable examples?
- **Skill Relevance**: Does every section support the parent skill's purpose?

**Novel detail follow-up**: When a skill or reference file scores 3 or higher on novelty, a separate follow-up call identifies which specific details are novel (proprietary APIs, internal conventions, unpublished workflows, etc.) in 1-2 sentences. This gives human reviewers a targeted signal for fact-checking without inflating novelty scores. The follow-up is non-fatal; if it fails, scores are returned normally. The result appears as "Novel details:" in text output and as `novel_info` in JSON/cached output.

## Stability

This project follows [semantic versioning](https://semver.org/) starting at v1.0.0.

**Stable packages** (breaking changes only in major releases):

- `orchestrate`
- `evaluate`
- `structure`
- `content`
- `contamination`
- `links`
- `skill`
- `skillcheck`
- `report`
- `types`

**Experimental packages:**

- `judge` — This package is under active development as the LLM scoring approach evolves. Its API may change in minor releases without a major version bump. The package doc comment includes an `EXPERIMENTAL` notice.

**What counts as a breaking change** (for stable packages):

- Removing or renaming an exported symbol (function, type, constant, variable)
- Changing a function or method signature (parameters, return types)
- Incompatible behavior changes that would break existing callers

**Deprecation process:**

Deprecated symbols are annotated with `// Deprecated:` comments following [Go convention](https://go.dev/wiki/Deprecated). Deprecated symbols are kept for at least one minor release after their replacement ships, giving consumers time to migrate before removal.

## Related: Cross-Platform Skill Loading Research

`skill-validator` checks that your skill is well-structured. But structure is only half the story. How platforms actually *load* your skill (what they read, when they read it, what they show the model) varies across the 25+ platforms that support Agent Skills, and no one has systematically tested it.

The [Agent Skill Implementation](https://github.com/agent-ecosystem/agent-skill-implementation) project is building empirical data about platform loading behavior through community testing. If you're distributing skills that need to work across platforms, your observations are valuable. See [agentskillimplementation.com](https://agentskillimplementation.com) for details.

## Development

Run lint and tests locally before pushing:

```
golangci-lint run ./...
go test -race ./... -count=1
```

CI runs both checks on every pull request. Install
[golangci-lint](https://golangci-lint.run/welcome/install/) if you don't
have it already.
