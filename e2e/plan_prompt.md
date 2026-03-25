# Planning Prompt

Understand the repository, the issue contract, and the verification target before planning.

## Required Outputs

- Create a non-empty `feature_list.json`.
- Create and initialize `progress.txt`.
- Only create those two issue artifact files.
- Do not create extra planning files in the repository.

## Planning Goal

Break the work into executable features for a minimal Go HTTP key-value server.

## Final Contract

The final implementation must support:

- `GET /set?key=a&value=b` -> HTTP 200 with body `ok`
- `GET /get?key=a` -> HTTP 200 with body `b`
- Missing keys -> HTTP 404
- `go run .` as the server entrypoint
- `PORT` environment variable with fallback to `8080`

## Planning Rules

- Include at least one feature covering self-verification by the coding agent.
- Keep the plan minimal and dependency-free.
- `feature_list.json` must be exactly a JSON array.
- Each feature item must have exactly these fields:
  - `id`
  - `title`
  - `description`
  - `priority`
  - `passes`
  - `notes`
- Initialize every `passes` value to `false`.
