# Stale issues and pull requests

`actions/stale` only processes issues and PRs labeled `needs information`.
It warns after 30 inactive days, adds `information overdue`, and can close after seven more.
Any activity, including maintainer or bot updates, can reset the timer. Issues close as
`not_planned`. PR branches are retained, and nothing is automatically reopened.

## Usage

- Maintainers apply `needs information` after requesting author details, never for PRs awaiting review.
- Remove it when satisfied or to stop processing. Activity clears only `information overdue`.
- Never apply the overdue label manually. Clear both labels before reopening or re-enrolling.

## Rollout

Ensure Issues is enabled and both labels exist. Both switches default to disabled and require
the exact string `true`.

- `ISSUE_LIFECYCLE_WRITES_ENABLED` enables warnings and label changes. Otherwise, runs are previews.
- `ISSUE_AUTO_CLOSE_ENABLED` also enables closure, but only when lifecycle writes are enabled.

Runs are daily. Manual `dry_run` defaults to checked and overrides both switches without changing
issues, PRs, or saved progress.

Review preview logs for at least one week before enabling writes. Observe warnings and their
grace period before enabling closure. Preview again first, since existing warnings may be eligible.
