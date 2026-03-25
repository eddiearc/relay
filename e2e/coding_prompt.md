# Coding Prompt

Implement the requested Go HTTP key-value server in `WORKDIR_PATH`.

## Contract

- `GET /set?key=a&value=b` returns HTTP 200 with body `ok`
- `GET /get?key=a` returns HTTP 200 with body `b`
- Missing keys return HTTP 404
- Use an in-memory map
- Support `PORT` from the environment with fallback to `8080`
- `go run .` must start the server
- Do not add third-party dependencies

## Required Self-Verification

Before finishing this coding run, you must verify the behavior yourself:

- Start the server locally
- Call `/set` and `/get`
- Confirm the responses match the contract
- Stop the server cleanly

## Relay Requirements

- Append the verification steps and results to `PROGRESS_PATH`
- Update `FEATURE_LIST_PATH` based on actual completion
- Keep `FEATURE_LIST_PATH` as a JSON array of items using exactly:
  - `id`
  - `title`
  - `description`
  - `priority`
  - `passes`
  - `notes`
- Commit repository changes before finishing
