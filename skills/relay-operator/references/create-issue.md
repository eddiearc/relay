# Create Issue

Use this reference when the user specifically needs deeper issue authoring guidance than the top-level `relay-operator` workflow provides.

## A Good Relay Issue Contains

- `goal`: one sentence describing the end state
- `description`: scope, constraints, non-goals, verification signals, and any existing clues
- any user-provided validation requirement, preserved verbatim when possible
- clear exclusions when the task boundary matters

## Feature Decomposition Rules

- one feature should map to one observable outcome
- `title` should describe a result, not an implementation step
- `description` should describe how to tell whether the result is achieved
- `priority` should reflect execution order or dependency order
- `passes` can become `true` only after verification
- `notes` should contain evidence, blockers, or residual risk

## Acceptance Criteria Rubric

Good acceptance criteria are observable from outside the model:

- a command passes: `go test ./...`, `pnpm test`, `cargo test`
- a build passes: `npm run build`
- a typecheck passes: `tsc --noEmit`
- an API returns an expected status code or response field
- a UI interaction produces an expected visible state
- a file or report is generated with expected contents
- a service startup produces an expected event or log signal

Bad acceptance criteria are vague and non-testable:

- "Implemented the logic"
- "Mostly done"
- "Code looks correct"
- "Handled the main cases"

## Required `feature_list.json` Shape

Relay requires `feature_list.json` to be a JSON array, and each item must use exactly these fields:

```json
[
  {
    "id": "feature-1",
    "title": "CLI prints the new summary section",
    "description": "Running the target command prints the summary section with non-empty values.",
    "priority": 1,
    "passes": false,
    "notes": ""
  }
]
```

## Recommended Writing Pattern

When the user gives you a vague request, rewrite it into:

- what result should exist when the work is done
- how a user or operator can verify that result
- which commands, behaviors, or outputs count as evidence
- what is explicitly out of scope

## Create the Issue

For a simple task:

```bash
relay issue add \
  --pipeline <pipeline-name> \
  --goal "Add X" \
  --description "Scope, constraints, verification commands, and non-goals."
```

For a larger task, prefer authoring `issue.json` and importing it:

```bash
relay issue import -file /path/to/issue.json
```

## Common Mistakes

- writing "implement X" without saying how to verify X
- mixing multiple outcomes into one feature
- making acceptance criteria depend on model narration instead of external evidence
- failing to preserve user-provided constraints or non-goals
