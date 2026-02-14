# v2/skills

This directory holds PicoClaw skills for the easyweb3 v2 PaaS.

## Layout

One skill per project/service:

- `v2/skills/<skill-name>/SKILL.md`

At runtime, PicoClaw loads skills from the workspace:

- `<picoclaw-workspace>/skills/<skill-name>/SKILL.md`

So the simplest deployment is a volume mount from `v2/skills` to `<workspace>/skills`.

## Conventions

- Skills are Markdown with YAML frontmatter (`name`, `description`).
- Skills call the PaaS via `exec easyweb3 ...` (not raw HTTP).
- Before running service operations, ensure you are authenticated:
  - `exec: easyweb3 auth login --api-key ...`
- Every state-changing action should also create a PaaS log entry (`easyweb3 log create ...`) so the web dashboard can trace agent behavior.

## Environment

Inside PicoClaw runtime (host or container), set:

- `EASYWEB3_API_BASE`: PaaS base URL (e.g. `http://easyweb3-platform:8080` in compose, or `http://127.0.0.1:18080` on host)
- `EASYWEB3_PROJECT`: project/service name (e.g. `polymarket`)

Token persistence is handled by `easyweb3` in `~/.easyweb3/credentials.json` by default.

