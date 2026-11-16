# Roadmap and milestones

[Project overview](../README.md) · [Documentation index](README.md) · [Detailed TODOs](todo.md)

The milestones group planned work by outcome. The order below is suggested priority, with no committed delivery dates. Each milestone links to its implementation checklist; completing one requires the stated verification as well as code and documentation.

| Milestone | Outcome | Completion criteria | Checklist |
| --- | --- | --- | --- |
| 1. Repeatable model simulation | Test common EcoFlow models without hardware | Model presets and deterministic scenarios exercise `OL`, `OB`, `OB LB`, recovery, and stale telemetry through the NUT server; document a controlled `upsmon` test setup | [Model simulation](todo.md#planned-model-simulation) |
| 2. Verified power status and shutdown | Establish when NUT clients should respond to reported power and battery states | Compare raw telemetry with hardware observations, then verify threshold, communication-loss, recovery, and shutdown behavior on a controlled Pi/NAS setup | [Power status and NUT shutdown](todo.md#deferred-power-status-and-nut-shutdown-verification) |
| 3. Guided setup | Get from discovery to a validated configuration | Discover devices, resolve and persist the user ID, preview configuration, and pass a connection/preflight check | [Guided setup](todo.md#first-priority-guided-setup) |
| 4. Clear status and recovery | Make connection progress, stale data, and failures understandable | Verify device progress and recovery states, actionable errors, diagnostics, and accessible dashboard behavior | [Device status](todo.md#next-priority-device-status-and-recovery) and [usability](todo.md#follow-up-usability-and-support) |
| 5. Notifications | Deliver useful state changes without blocking collection | Verify event routing, deduplication, retries, recovery, and credential redaction, starting with webhooks and SMTP | [Notifications](todo.md#planned-notifications) |

Model simulation can establish how the daemon and NUT clients respond to a scenario. Hardware tests must still establish whether real EcoFlow readings represent that scenario correctly. Full `upsmon` shutdown compatibility remains unverified; see the [current command coverage](nut-protocol.md#command-coverage).
