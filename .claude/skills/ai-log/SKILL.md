---
name: ai-log
description: Append an entry to the repo's AI-log.md recording which project files this AI session wrote, edited, moved or deleted. Use only at the end of a session that changed files in the project, or when the user says "log this", "update the AI log", or invokes /ai-log. Do not use for sessions that only answered questions, explained, planned, reviewed, or ran commands without changing project files.
---

# AI log

`AI-log.md` at the repo root is an append-only record of AI sessions that **changed the project**: one entry per session, newest at the bottom. Its job is to let a human (or the next AI) see *which files an AI touched and why* without reading diffs.

## Gate: only log project writes

Before writing anything, check whether this session wrote to the project. That means creating, editing, moving, renaming, or deleting any file inside the repo (not counting `AI-log.md` itself).

- **Yes** → continue to the steps below.
- **No** → **do not touch `AI-log.md`.** Questions, explanations, advice, plans, code reviews, builds, test runs, and deploys that changed no repo files never get an entry, and neither do "No project changes" bullets. If the user explicitly asked for a log entry, tell them in one line that nothing in the project changed, so there's nothing to log.

Files outside the repo, such as `~/.claude/` memory or the scratchpad, don't count as project writes.

## Steps

1. **Collect what this session changed.** Use your own memory of the session first; it is the source of truth for what *you* changed. Cross-check with:
   ```bash
   git status --porcelain
   git diff --stat
   date '+%Y-%m-%d %H:%M'
   ```
   Only log changes this session made. Ignore pre-existing dirty files you didn't touch.

2. **Find your existing entry, if any.** Search `AI-log.md` for `<!-- id: ${CLAUDE_SESSION_ID} -->`.
   - Found → update that entry in place: add new bullets, revise bullets for files you changed again. Keep its original heading time.
   - Not found → append a new entry at the end of the file.
   - If `AI-log.md` doesn't exist, create it with `# AI log` and a blank line as the first lines.

3. **Write the entry** in the format below, then confirm in one line which entry you added or updated.

Never edit or reorder other sessions' entries. The one exception is a rename or move of a path that old entries mention. In that case, update the stale paths and log that you did so.

## Entry format

```markdown
## YYYY-MM-DD HH:MM
<!-- id: <session id> -->
- `path/to/file.go` — what changed, as a terse fragment.
- `path/to/dir/` — what changed in this directory.
```

- **Heading:** local date and 24-hour time from `date '+%Y-%m-%d %H:%M'`, taken when you write the entry. No timezone and no seconds.
- **id comment:** the session id `${CLAUDE_SESSION_ID}`. If that substitution is empty or literal, omit the line. Never make one up.
- **Blank line** between entries, and none inside one.

### File bullets

- One bullet per file. If a change spans many files in one directory for one reason, use a single bullet for the directory with a trailing `/`.
- Format: `` `repo-relative/path` — description. `` using an em dash (—) with a space on each side.
- Description: a short fragment, not a sentence about yourself. Name the function, type, command, or flag involved, and say *what* plus a brief *why* when it isn't obvious. Wrap identifiers in backticks.
  - Good: `` `backend/store/store.go` — `MarkDue` so ListDue does not re-claim until the interval elapses. ``
  - Bad: `` `backend/store/store.go` — I updated this file to add some new functionality. ``
- Deleted: `` `path` — removed (reason). ``
- Moved or renamed: `` `old/` → `new/` `` inside the description, or log the destination path with "moved from …".
- Generated code is listed as its directory, e.g. `` `backend/protoAPI/subscription/` — generated stubs. ``
- Order bullets by importance or dependency (foundations first), not alphabetically.
- Don't list `AI-log.md` itself.

### Optional bullets (only inside an entry that already has file bullets)

- `- Model: <name> (via <tool>)` as the first bullet. Include it only when the work was done by something other than the default Claude Code model, or the user asks for it.
- Commits made this session: `` - Commit `abc1234` — "subject line". ``
- Builds, deploys, or migrations that shipped this session's changes. Say what ran, where, and the outcome, including failures and how they were fixed.
- Verification: `` - Verified both modules build (`go build ./...`) and all tests pass (`go test ./...`). `` Write this only if you actually ran them. If tests failed, say so.

## Style

- Plain, factual, past or present tense fragments. Use no first person ("I"), no filler words like "successfully", and no emoji.
- Write repo-relative paths, never absolute ones.
- Never include secrets, tokens, passwords, API keys, or the contents of `.env` files.
