# Monitor Relay

Use this reference when the user specifically needs deeper service and debugging guidance than the top-level `relay-operator` workflow provides.

## Startup Sequence

Always use this order:

1. run `relay serve --once` and confirm the issue, pipeline, and artifacts are valid
2. run `relay serve` in the foreground and confirm it polls, creates workspaces, and writes artifacts
3. only then move it into persistent background or supervised mode

## Mode Selection

- single pass for debugging: `relay serve --once`
- foreground validation: `relay serve`
- lightweight background process: `nohup relay serve >> ~/.relay/logs/serve.log 2>&1 &`
- production-style supervision: `systemd`, `launchd`, or another service manager

## Service Rules

- keep a dedicated stdout/stderr log for `relay serve`
- do not casually start multiple `relay serve` processes against the same state directory
- if you need multiple workers, define state isolation or queue isolation first
- prefer a supervisor that can restart the service and expose service logs

## Check Relay State First

```bash
relay issue list
relay status -issue <issue-id>
relay report -issue <issue-id>
```

Pay attention to:

- whether `status` is still `todo`, `planning`, `running`, `done`, or `failed`
- whether `current_loop` is increasing
- whether `last_error` is non-empty
- whether the artifact path exists

## Read Issue Artifacts Directly

```bash
cat ~/.relay/issues/<issue-id>/issue.json
cat ~/.relay/issues/<issue-id>/feature_list.json
tail -n 200 ~/.relay/issues/<issue-id>/progress.txt
tail -n 200 ~/.relay/issues/<issue-id>/events.log
ls -la ~/.relay/issues/<issue-id>/runs
```

## Common Diagnostic Signals

- `events.log` contains `planning validation failed`
  - usually `feature_list.json` is empty, malformed, or uses the wrong fields
- `current_loop` increases but `feature_list.json` barely changes
  - usually the prompts are weak or the coding agent is not updating artifacts from real evidence
- `last_error` is non-empty
  - read `relay report -issue <issue-id>` and the latest run stderr immediately
- an issue stays `todo` and no new events appear
  - usually `relay serve` is not running, or it is running against the wrong state directory or environment
- code changed but all `passes` values remain `false`
  - usually verification never happened, or the feature descriptions are too vague to judge

## Check Host-Level Service State

Confirm the process exists:

```bash
ps -ef | rg "[r]elay serve"
```

On macOS:

```bash
launchctl list | rg relay
log show --last 1h --predicate 'process == "relay"'
```

On Linux with `systemd`:

```bash
systemctl status relay
journalctl -u relay -n 200 --no-pager
```
