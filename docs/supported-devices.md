# Supported devices and current scope

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment options](../deploy/README.md)

Implemented:

- TCP NUT-like server on port `3493`
- optional local web status/control server
- multiple virtual UPS devices
- direct `eco-ble` provider for modern EcoFlow power stations
- `upsc`-style commands:
  - `VER`
  - `LIST UPS`
  - `LIST VAR <ups>`
  - `GET VAR <ups> <var>`
  - `GET UPSDESC <ups>`
- simple `USERNAME` / `PASSWORD`
- mock provider
- JSON directory provider for integration testing

Implemented `eco-ble` scope:

- Linux/BlueZ runtime via `tinygo.org/x/bluetooth`
- configured-device discovery by `devices[].mac`
- auth with direct `user_id` or one-time cloud bootstrap from `email/password`
- session modes `0`, `1`, and `7`
- V2 families:
  - DELTA 2
  - DELTA 2 Max / Max S
  - RIVER 2 / Max / Pro
- V3 / `0x13` families:
  - DELTA 3 family
  - DELTA Pro 3 family
  - DELTA Pro Ultra
  - RIVER 3 family
- stale-device handling with `ups.status=WAIT`

Still not implemented:

- legacy V1 devices
- V4 protocol devices
- non-power-station product classes such as WAVE, generators, SHP, STREAM, PowerOcean, and similar
- device control writes such as AC/DC switching or charge-limit updates
- full NUT protocol coverage
- TLS
- verified `upsmon` compatibility in every shutdown mode

This is still an early standalone replacement, but it now includes a direct read-only EcoFlow BLE path instead of requiring an external collector.
