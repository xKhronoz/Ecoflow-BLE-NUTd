# TODO and UX improvements

[Project overview](../README.md) · [Documentation index](README.md)

This checklist tracks setup automation and user experience improvements. Unchecked items are planned work, not available features. Sections are ordered by suggested priority; no delivery dates are committed.

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

## Completion criteria

For each completed item, update the relevant setup or configuration guide and verify the user-visible behavior. Use the mock provider for interface work and real BLE hardware for discovery, authentication, and reconnect behavior.
