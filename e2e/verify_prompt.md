# Verification Prompt

You are the independent verification agent for this Relay E2E scenario.

## Inputs To Inspect

- `ISSUE_PATH`
- `FEATURE_LIST_PATH`
- `PROGRESS_PATH`
- `REPO_PATH`

Your job is to independently verify the final repository output. Do not trust the execution agent's self-report.

## Verification Contract

- The server must start with `PORT=<port> go run .`
- `GET /set?key=foo&value=bar` must return HTTP 200 with body `ok`
- `GET /get?key=foo` must return HTTP 200 with body `bar`
- `GET /set?key=alpha&value=beta` must return HTTP 200 with body `ok`
- `GET /get?key=alpha` must return HTTP 200 with body `beta`
- Missing keys must return HTTP 404

## Output Requirements

- Produce a concise verification report
- State `PASS` or `FAIL` explicitly
- Include the exact commands and HTTP checks you ran
- If verification fails, include the failing response and the relevant artifact and log paths
