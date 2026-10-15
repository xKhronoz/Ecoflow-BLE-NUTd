# TODO and UX improvements

[Project overview](../README.md) · [Documentation index](README.md)

This checklist tracks setup automation, user experience improvements, and deferred verification. Unchecked items are planned work, not available features or verified behavior. The setup and UX sections are ordered by suggested priority; no delivery dates are committed.

## First priority: guided setup

- [ ] Automatically discover nearby EcoFlow BLE devices. Show advertisement name, address, detected model when available, and whether the device profile is supported; let the user rescan and select devices instead of typing addresses.
- [ ] Generate a configuration file from selected devices. Preview the configuration, validate unique NUT names and required fields, and offer a download or explicit save without overwriting an existing file silently.
- [x] Resolve the EcoFlow account `user_id` from email and password through cloud login. Already implemented when `provider.auth.user_id` is empty; the result currently stays in memory.
- [ ] Provide a one-time cloud login flow that persists the resolved `user_id` in the configuration for future starts. Explain that telemetry stays local, avoid retaining the cloud password in the generated file, protect the saved file with restrictive permissions, and report read-only paths clearly.
- [ ] Add a guided setup command or wizard that connects discovery, account login, configuration preview, and a connection check. Explain the difference between EcoFlow account credentials and the daemon's NUT/web credentials.
- [ ] Add a preflight check for the configured adapter, Bluetooth permissions, configuration, and selected devices. Report each failure with a specific next step before starting collection.

## Next priority: device status and recovery

- [ ] Show per-device connection progress: scanning, connecting, authenticating, receiving telemetry, and retrying. Include the last failure and next retry time so a running collector does not imply every device is connected.
- [ ] Turn common scan, connection, and authentication failures into actionable messages. Distinguish an undiscovered device, unavailable adapter, permission failure, and rejected login; link to the relevant troubleshooting instructions.
- [ ] Make stale telemetry prominent. Show the last successful reading time, a clear stale/offline label, and a plain-language explanation of NUT `WAIT`; do not present cached readings as live values.
- [ ] Summarize battery charge, estimated runtime, input/output power, and power source with readable labels and units. Keep raw NUT variables available in an expandable details section.
- [ ] Add an empty state when no device readings exist. Explain whether the collector is starting, paused, or waiting for a device, and show the next useful action.
- [ ] Rename collector controls to “Resume collection” and “Pause collection,” disable actions that already match the current state, and explain that pausing collection leaves the NUT server running with `WAIT` data.
- [ ] Separate page-refresh failures from collector/device failures. Show when the page last fetched status successfully and offer a manual refresh action alongside automatic polling.

## Follow-up: usability and support

- [ ] Add a sanitized diagnostics download containing daemon version, host platform, provider state, and recent error categories. Exclude credentials, tokens, user IDs, serials, and device addresses by default; preview the contents before export.
- [ ] Provide a copyable NUT connection example using the configured device name and server address, plus a connection-check result. Keep passwords out of examples and distinguish server reachability from live BLE telemetry.
- [ ] Verify the status page on small screens and with keyboard/screen-reader navigation. Cover long device names and errors, visible focus, readable contrast, accessible table labels, and status announcements without announcing every poll.
- [ ] Consolidate troubleshooting into a symptom-to-action guide for scan timeouts, login failures, stale readings, and container/native setup. Correct credential-encoding guidance to distinguish a plain-text configuration password from the encoded cloud API payload.
- [ ] Document platform-specific discovery behavior and supported runtime limits, including how device identifiers differ between Linux and macOS. Provide verified discovery steps for each documented platform.

## Planned: notifications

To design notification channel configuration and provider coverage. The providers and URL schemes below are implementation candidates, not currently supported configuration; validate each integration before documenting its final syntax.

- [ ] Add named, individually enabled notification channels with per-device, event, and severity selection. Start with generic JSON webhooks and SMTP email, then extend provider coverage.
- [ ] Support SMTP settings for server, port, sender, recipients, authentication, and TLS. Provide a test-email action and actionable delivery errors without exposing credentials.
- [ ] Support one or more destination URLs per webhook channel, entered one per line. Define and validate provider-specific URL schemes, preserve required paths/query parameters, handle percent-encoded credentials, and show which destinations succeeded or failed.
- [ ] Evaluate provider adapters for Discord, Slack, Mattermost, Telegram, ntfy, Gotify, Pushover, Bark, Signal via signal-cli-rest-api, Microsoft Teams via Workflows, Zabbix via `history.push`, and generic JSON. Document any external service requirements and verify payload formats and authentication for each adapter.
- [ ] Support Apprise API passthrough for providers without native adapters, including authenticated self-hosted endpoints. Explain the separate Apprise service requirement.
- [ ] Add optional MQTT notification publishing with TCP/TLS, broker authentication, configurable topic, QoS, and retain behavior. Publish structured JSON events and document URL escaping for topics; keep MQTT optional for normal BLE/NUT operation.
- [ ] Define device-health alerts for stale telemetry, sustained disconnects, authentication failures, and recovery. Add configurable battery-charge/runtime threshold alerts with hysteresis, plus explicit collector pause/resume events that distinguish intentional pauses from failures.
- [ ] Add power-transition alerts after the deferred `OB`/`OL` mapping tests establish their accuracy. Label events as reported power state rather than claiming a confirmed mains outage. Keep daemon-observed states distinct from NUT-client shutdown decisions; only notify about shutdown initiation/completion when the relevant integration can confirm it.
- [ ] Prevent notification floods with transition-based events, configurable debounce/cooldown, deduplication, and recovery notifications. Include device name, timestamp, severity, relevant readings, and a dashboard link when configured.
- [ ] Keep delivery asynchronous with bounded queues, request timeouts, capped retries/backoff, and per-destination failure isolation so slow channels cannot block BLE collection or NUT responses. Define queue-overflow and restart behavior explicitly.
- [ ] Add channel management and test-send controls to the web UI, with masked secrets, clear validation, last delivery status, and sanitized error details. Keep tokens, passwords, and secret-bearing destination URLs out of status APIs, logs, and diagnostics exports.
- [ ] Evaluate provider-specific options such as Bark urgency, sound, grouping, and tap-through links. Document defaults and make disruptive notification modes explicit choices.
- [ ] Verify routing, fan-out, duplicate suppression, recovery alerts, invalid URLs, timeouts, retry exhaustion, SMTP/TLS failures, and credential redaction using mock delivery endpoints. Add setup examples and a supported-provider matrix once implemented.

## Deferred: power status and NUT shutdown verification

These investigations are deferred from [PR #3](https://github.com/xKhronoz/Ecoflow-BLE-NUTd/pull/3). DELTA 3 authentication and telemetry have been reported working, but power-status accuracy and end-to-end shutdown behavior remain unverified.

- [ ] Test DELTA 3 `OB`/`OL` accuracy with mains connected at the charge limit, below the limit while charging, disconnected, and reconnected. Compare no load, a small USB/DC load, and a steady AC-outlet load; record which ports are enabled and supplying power. Capture timestamped `upsc` readings, EcoFlow app observations, firmware version, and daemon logs after each condition settles.
- [ ] Validate power-status mapping against raw telemetry. Capture both the presence and values of `plug_in_info_ac_in_flag` and `plug_in_info_ac_charger_flag`, AC-specific input watts, and total input watts across the test conditions. Establish what the flags mean before changing the decoder; distinguish mains availability, charging activity, and battery discharge. Cover zero input watts at the charge limit, solar/DC input without mains, and omitted fields in partial updates. Add regression tests for the mapping supported by hardware evidence.
- [ ] Verify NUT shutdown behavior end to end on a controlled Pi/NAS setup. Check `upsmon.conf`, any `upssched.conf`/command script, and NAS shutdown policy; exercise `OB`, configured low-battery/runtime thresholds, brief communication loss and recovery, and sustained communication loss. Confirm intended `OB LB`/`FSD` handling, graceful shutdown, and absence of unintended shutdowns during normal operation. Record NUT version, configuration, and logs; use a noncritical load or disable shutdown actions for status-only tests.
- [ ] Investigate the reported improvement after setting `upsmon` `DEADTIME` to 45 seconds. Correlate failed status polls and shutdown logs with the previous timeout and polling intervals. Treat the report as evidence about communication tolerance, not proof of correct `OB`/`OL` mapping; document any verified configuration guidance and remaining protocol limitations in the [NUT guide](nut-protocol.md).

## Completion criteria

For each completed item, update the relevant setup or configuration guide and verify the user-visible behavior. Use the mock provider for interface work and real BLE hardware for discovery, authentication, and reconnect behavior.
