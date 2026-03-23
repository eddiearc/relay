# Relay

Relay is a goal-driven Go CLI for supervising short-lived AI worker runs.

The initial scaffold sets up a small binary entrypoint and placeholder commands
for the core run lifecycle:

- `start`
- `status`
- `report`
- `kill`

The runtime and agent control loop will be added on top of this skeleton.
