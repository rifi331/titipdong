# Git hooks

## pre-commit

Blocks accidental commits containing:

- Private network IPs (`192.168.x.x`, `10.x.x.x`, `172.16-31.x.x`)
- Common API key patterns (OpenAI `sk-`, GitHub `ghp_`, AWS `AKIA`, Slack `xox`, ...)
- Postgres URLs with literal passwords (instead of `__DB_PASSWORD__` placeholder)
- Hardcoded 32+ char hex secrets (looks like real SESSION_SECRET / password)

## Install (run once per clone)

```bash
git config core.hooksPath scripts/git-hooks
chmod +x scripts/git-hooks/pre-commit   # Linux/macOS only; Windows skips this
```

After install, every `git commit` runs the hook automatically. If it finds a
suspicious pattern, the commit is rejected.

## Bypass for a single commit

Only when you're sure the flagged content is safe (e.g. a test fixture, a
publicly documented example):

```bash
git commit --no-verify
```

## Add new patterns

Edit `pre-commit` and add a line to the `PATTERNS` array:

```bash
'your-regex-here	"description shown on match"'
```

The separator between regex and description is a literal TAB character.
