# Security Policy

## Reporting a vulnerability

Use the repository's [private vulnerability reporting page](https://github.com/xKhronoz/Ecoflow-BLE-NUTd/security/advisories/new) if GitHub offers that option. If private reporting is unavailable, open an issue requesting a private contact without including credentials, exploit code, or sensitive device information.

Include the affected release or commit, deployment method, exposed interface, authentication settings with secrets removed, reproduction steps, and expected impact. For BLE issues, include the device family and firmware version. Redact account passwords, user IDs, device serials, MAC addresses, and tokens from shared logs and configuration.

A formal security support window, backport policy, and response deadline have not been established. Report the exact affected version and whether the issue also exists on `main`; reports affecting older releases are still useful.

## System and scope

Ecoflow-BLE-NUTd is a Linux daemon that exposes EcoFlow power-station telemetry through a NUT-compatible TCP server and an optional HTTP status/control interface. The `eco-ble` provider communicates through the host's BlueZ system D-Bus service. An optional EcoFlow cloud login resolves the app account's user ID; telemetry collection then uses BLE. The `mock` and `json-dir` providers support local testing and integration.

Security review covers the daemon, BLE framing and authentication, telemetry parsing, NUT and web interfaces, configuration handling, dependencies, installation scripts, container deployment, and release workflows. Relevant code is under `cmd/` and `internal/`; deployment and build inputs are under `deploy/`, `scripts/`, `systemd/`, and `.github/workflows/`.

## Threat model and trust boundaries

Protect account and NUT credentials, the daemon's host privileges, collector availability, and the integrity and freshness of UPS telemetry. Incorrect telemetry can affect decisions made by downstream monitoring or shutdown clients.

- **Network clients:** treat NUT commands, HTTP requests, and browser-originated traffic as untrusted, including on a LAN or loopback interface. Configured credentials grant access to the supported interface; the same credentials protect the web UI and its collector controls. There are no separate administrator and monitoring roles.
- **BLE peers:** advertisements, serials, public keys, frames, and telemetry are untrusted. A configured MAC address selects a device; it does not authenticate the peer. Consider an attacker within BLE range who can impersonate advertisements or send malformed responses.
- **Host administration:** an operator controls configuration, installed binaries, service files, and deployment settings. Administrator control over those files is distinct from a lower-privilege user's ability to modify them unexpectedly. Assess filesystem ownership and permissions before deciding that input is trusted.
- **JSON inputs:** the operator chooses the input directory and authorized telemetry producer. Files still require safe parsing. A lower-privilege writer must not gain code execution or access beyond its intended telemetry role.
- **Cloud and host services:** EcoFlow login responses and BlueZ/D-Bus messages cross external boundaries. The Docker service mounts the host system bus and runs as UID 0 with capabilities dropped; those settings do not eliminate the authority granted by host D-Bus policies.

## Security invariants

These are required review properties, not a claim that every control has been independently verified:

- When authentication is configured, protected reads and collector enable/disable operations must require the configured credentials. Browser requests must not bypass protection or perform unauthorized state changes. Deliberately empty credentials disable authentication and must be assessed in that configuration.
- Untrusted packet lengths, public keys, protobufs, JSON, and network commands must be validated before indexing, allocation, or use. Reject unsupported or oversized input with an error; do not allow length truncation, panics, or disproportionate resource use. Include 32-bit ARM when assessing arithmetic.
- BLE authentication and session failures must not authorize a peer merely because unrelated telemetry arrives. Reconnect, cancellation, and collector disable operations must not leave unintended active connections.
- Stale or disconnected devices must not be presented as fresh, healthy UPS telemetry. Device input must not inject executable HTML or unintended NUT responses.
- Credentials and session secrets must not appear in ordinary logs, web responses, build contexts, images, or release archives. Credential-bearing configuration must have appropriately restricted host permissions.
- Untrusted network or device data must not cause arbitrary file access, command execution, or host D-Bus operations outside the implemented BLE collector behavior.
- Untrusted contributions must not obtain release credentials or modify published artifacts through CI. Release and deployment changes must preserve the intended privilege and credential boundaries.

## Reportable findings and severity

Report authentication bypasses, credential disclosure, unauthorized collector control, unsafe parsing, meaningful telemetry manipulation, reachable denial of service, host privilege escalation, and CI or release compromise.

Explain the actor, controlled input, required access, affected configuration, guards, and consequence. Assess supported alternate configurations as well as defaults. LAN deployment, optional authentication, or a container boundary alone does not invalidate a finding.

Scanner severity is a starting point. An arithmetic warning in an encoder needs a reachable source capable of supplying the problematic size before it establishes attacker-triggered overflow. Small internally generated messages are relevant counterevidence; they do not justify removing bounds checks. Dependency presence or a weak-primitive warning alone also does not establish a specific exploit.

## Known limitations and operational controls

- **Transport:** NUT has no TLS/STARTTLS, and the web server uses HTTP Basic authentication without built-in TLS. Restrict listeners to intended clients using firewall rules, loopback bindings, or a protected tunnel. Docker publishes NUT on all host interfaces by default and the web port on loopback; operators can change both.
- **Protocol cryptography:** EcoFlow requires MD5 for specific IV, session-key, and authentication-token derivations and uses the legacy secp160r1 curve for type 7. Replacing these locally with different algorithms breaks firmware interoperability. These are known protocol limitations, not an assertion that the cryptography is strong. See [BLE protocol cryptography](docs/ble-authentication.md#protocol-cryptography).
- **Credentials:** configuration can contain plaintext account credentials and NUT credentials. After obtaining an app user ID, an operator can omit the cloud password; the user ID and configuration still require protection. Base64-encoding the cloud login password is not encryption.
- **Compatibility:** device control writes, complete NUT protocol parity, and verified end-to-end `upsmon` shutdown semantics are not implemented. A missing advertised-unsupported feature alone is a compatibility issue; a reachable security failure in an implemented path remains reportable.

No blanket finding exclusions or accepted-risk dismissals are established by this policy. Protocol compatibility must not automatically dismiss MD5 alerts, suppress related findings, or excuse an exploitable authentication or confidentiality failure. Maintainers must review specific risks and any proposed exclusions separately.

See the [Docker deployment guide](deploy/docker/README.md), [native deployment guide](docs/deployment.md), and [configuration guide](docs/configuration.md) for operational settings.
