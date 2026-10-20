# Contributing

## Commit messages

Use this format:

```text
<type>: <description>
```

Allowed types:

| Type | Use for |
| --- | --- |
| `feat` | New user-facing capability |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `test` | Tests only |
| `refactor` | Code change without behavior change |
| `chore` | Maintenance, dependencies, tooling |
| `ci` | CI/CD changes |

Rules:

- Keep the type lowercase.
- Write the description in the imperative mood, lowercase.
- Do not add a period at the end.
- Keep the subject line under 72 characters.
- Do not use scopes such as `feat(web): ...`.
- Keep one logical change per commit.

Examples:

```text
feat: add SSE progress stream
fix: handle missing company profile
docs: describe the news API format
test: cover SSE reconnect
refactor: extract agent client
chore: update Go dependencies
ci: run ESLint and Vitest
```
