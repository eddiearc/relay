# AGENTS

## Working Norms

- Treat Relay as an execution harness first: prefer stable CLI flows, persisted state, and verifiable outcomes over ad-hoc one-off instructions.
- Use the repository docs and CLI help as the source of truth for operational details.

## E2E-First Validation

- For any meaningful new feature, behavior change, or bug fix, first evaluate whether the built-in `relay-e2e` skill can cover the change end to end.
- When the change is a good E2E candidate, prefer validating it with `relay-e2e`, not only with unit tests or manual spot checks.
- When E2E validation is viable, add or update a reusable scenario under `e2e/` so the case can be rerun later.
- Reusable E2E cases should follow the repository contract documented in `e2e/README.md` and keep scenario inputs/prompts focused on the behavior being verified.
- If a change is not a good fit for E2E coverage, say so explicitly and briefly explain why before falling back to narrower verification.
