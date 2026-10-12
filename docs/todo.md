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

## Deferred: power status and NUT shutdown verification

These investigations are deferred from [PR #3](https://github.com/xKhronoz/Ecoflow-BLE-NUTd/pull/3). DELTA 3 authentication and telemetry have been reported working, but power-status accuracy and end-to-end shutdown behavior remain unverified.

- [ ] Test DELTA 3 `OB`/`OL` accuracy with mains connected at the charge limit, below the limit while charging, disconnected, and reconnected. Compare no load, a small USB/DC load, and a steady AC-outlet load; record which ports are enabled and supplying power. Capture timestamped `upsc` readings, EcoFlow app observations, firmware version, and daemon logs after each condition settles.
- [ ] Validate power-status mapping against raw telemetry. Capture both the presence and values of `plug_in_info_ac_in_flag` and `plug_in_info_ac_charger_flag`, AC-specific input watts, and total input watts across the test conditions. Establish what the flags mean before changing the decoder; distinguish mains availability, charging activity, and battery discharge. Cover zero input watts at the charge limit, solar/DC input without mains, and omitted fields in partial updates. Add regression tests for the mapping supported by hardware evidence.
- [ ] Verify NUT shutdown behavior end to end on a controlled Pi/NAS setup. Check `upsmon.conf`, any `upssched.conf`/command script, and NAS shutdown policy; exercise `OB`, configured low-battery/runtime thresholds, brief communication loss and recovery, and sustained communication loss. Confirm intended `OB LB`/`FSD` handling, graceful shutdown, and absence of unintended shutdowns during normal operation. Record NUT version, configuration, and logs; use a noncritical load or disable shutdown actions for status-only tests.
- [ ] Investigate the reported improvement after setting `upsmon` `DEADTIME` to 45 seconds. Correlate failed status polls and shutdown logs with the previous timeout and polling intervals. Treat the report as evidence about communication tolerance, not proof of correct `OB`/`OL` mapping; document any verified configuration guidance and remaining protocol limitations in the [NUT guide](nut-protocol.md).

## Completion criteria

For each completed item, update the relevant setup or configuration guide and verify the user-visible behavior. Use the mock provider for interface work and real BLE hardware for discovery, authentication, and reconnect behavior.
