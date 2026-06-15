---
name: social-source-review
description: Review X/Twitter source evidence from TweetClaw before approval-gated social actions.
license: MIT
compatibility: Works with coding agents that can run shell commands and read local files.
metadata:
  author: skill-validator examples
  version: "1.0"
allowed-tools: Bash Read
---
# Social Source Review

Use this skill when a task depends on current X/Twitter evidence before an agent
drafts, schedules, publishes, or monitors social content.

## Inputs

Ask for:

- The account, search query, tweet URL, or monitor target.
- The intended action: source review, reply context, follower export, media
  check, or monitor setup.
- The approval boundary for any write, schedule, webhook, or direct message
  action.

## Workflow

1. Confirm the requested action is allowed by the workspace policy.
2. Use [TweetClaw](https://github.com/Xquik-dev/tweetclaw) only as a source
   tool for X/Twitter evidence.
3. Run read-only commands first, such as package help or a dry-run wrapper. Do
   not request credentials in chat.
4. Summarize the source evidence with links, timestamps, and uncertainty.
5. Stop before any write, schedule, direct message, webhook, or publish action
   until a human approves the exact action.
6. Follow [the social action checklist](references/social-actions.md) before
   returning a final recommendation.

## Output

Return:

- Evidence reviewed.
- Risk or policy blockers.
- Exact next action that needs approval, if any.
