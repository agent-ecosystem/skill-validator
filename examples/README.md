# Examples

This directory contains examples that extend skill-validator for common
workflows. They are not part of the tool itself; copy and adapt them to fit
your project.

- **review-skill** — Use during local development to iterate on a skill with
  your coding agent before requesting a human review.
- **ci** — Use when publishing skills or adding them to a project to enforce
  a minimum quality bar on every pull request.
- **social-source-review**: Validate a realistic skill that gates X/Twitter
  source review before social actions.

## review-skill

An [Agent Skill](https://agentskills.io/) that walks a coding agent through a
full skill review: structural validation, content checks, and LLM-as-judge
scoring. Use it during local development to iterate on a skill before
publishing. With this skill, the coding agent can work with the skill author to
improve the skill content before requesting a human review.

### What it does

1. Checks prerequisites (skill-validator binary, API keys)
2. Runs `skill-validator check` for structural validation
3. Reviews content for examples, edge cases, and scope-gating
4. Optionally scores the skill with an LLM judge (Anthropic, OpenAI, any
   OpenAI-compatible endpoint, or the Claude CLI)
5. Supports cross-model comparison to validate scores across model families
6. Presents a summary with prioritized action items and a publish recommendation

### Setup

1. Copy the `review-skill/` directory into your project's skill directory (or
   wherever your agent loads skills from). For Claude, for example, this is
   `.claude/skills/`.
2. Install the skill-validator tool. If it's not already installed, the skill
   contains install instructions to walk the agent through helping a skill author
   set up their environment.
3. For LLM scoring, set the relevant API key:
   - Anthropic: `export ANTHROPIC_API_KEY=sk-ant-...`
   - OpenAI: `export OPENAI_API_KEY=sk-...`
   - OpenAI-compatible: `export OPENAI_API_KEY=...` (some endpoints accept a
     placeholder) and provide the `--base-url` when prompted.
   - Claude CLI: No API key needed — uses the locally authenticated `claude`
     binary (e.g. via a company or team subscription). Note: scores may be less
     consistent than API-based providers because the CLI loads local context
     (CLAUDE.md, memory) into each call.
4. Add `.score_cache/` to your `.gitignore`. LLM scoring caches results inside
   each skill directory, and these should not be committed.
5. Ask your agent to review a skill. The skill stores configuration in
   `~/.config/skill-validator/review-state.yaml` so subsequent runs skip
   prerequisite checks.

## ci

A GitHub Actions workflow and companion script that validate new or changed
skills on every pull request. Use it to enforce a minimum quality bar before
skills are merged. Use when publishing official skills for other people to
use, or before adding skills to your own repo or personal coding agent setup.

### What it does

- Detects which skill directories changed in a PR (via `git diff`)
- Runs `skill-validator check --strict` on each changed skill
- Writes a markdown report to the GitHub Actions job summary
- Emits inline PR annotations for errors and warnings
- Fails the workflow if any skill has errors or warnings (`--strict` mode)

### Setup

1. Copy `.github/workflows/validate-skills.yml` and
   `.github/scripts/validate-skills.sh` into your repository's `.github/`
   directory.
2. Edit the `SKILLS_DIR` env var in the workflow to match the directory where
   your skills live (defaults to `skills`).
3. Update the `paths` filter under `on.pull_request` to match the same
   directory.
4. Ensure the script is executable: `chmod +x .github/scripts/validate-skills.sh`

The workflow installs skill-validator from source on each run. No API keys or
external services are required; it runs structural validation only.

## social-source-review

A compact sample Agent Skill package that shows how social-data skills can keep
read-only source review separate from approval-gated write actions. It uses
TweetClaw as the concrete X/Twitter source tool and includes a checklist for
credentials, evidence, and human approval boundaries.

Run:

```sh
go run ./cmd/skill-validator check examples/social-source-review
```
