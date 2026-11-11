# NUT variables and protocol coverage

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment options](../deploy/README.md)

## Variable mapping

| EcoFlow reading             | NUT variable      |
| --------------------------- | ----------------- |
| battery percent             | `battery.charge`  |
| estimated seconds remaining | `battery.runtime` |
| AC input present            | `ups.status=OL`   |
| no AC input                 | `ups.status=OB`   |
| low battery/runtime         | append `LB`       |
| input watts                 | `input.power`     |
| output watts                | `output.power`    |

### Further variables from the property profiles

The RIVER 3, DELTA 3 and DELTA Pro 3 families push a `DisplayPropertyUpload` message about once a second; the profiles map what it carries onto standard NUT names. A model yields the fields its message has, nothing is invented for a missing one.

| EcoFlow reading (display message)        | NUT variable                                          |
| ---------------------------------------- | ----------------------------------------------------- |
| AC input frequency, watts                | `input.frequency`, `input.realpower`                  |
| total output watts                       | `ups.realpower`                                       |
| rated AC output (`rated_power_watts` when the device reports none) | `ups.realpower.nominal`     |
| hottest cell                             | `battery.temperature`                                 |
| design capacity (Ah)                     | `battery.capacity.nominal`                            |
| discharge floor from the app             | `battery.charge.low`                                  |
| pack charging / discharging (`pow_get_bms`, else input minus output) | append `CHRG` / `DISCHRG`; `battery.charger.status` |
| one outlet per port                      | `outlet.N.desc`, `outlet.N.realpower`, `outlet.N.status` |

Outlet numbers are fixed per port so a reading keeps its meaning across models and partial updates: 1 AC, 2 AC 2 (second AC group), 3-5 USB-C, 6-7 USB-A, 8 12 V car socket, 9 12 V Anderson. A port the model lacks never appears; the Anderson port appears only when the device reports a second 12 V group, since it shares the 12 V switch with the car socket.

Two variables are derived after partial updates are merged, so the readings behind them need not arrive in the same message: `ups.load` is `ups.realpower` as a percentage of `ups.realpower.nominal`, and `ups.temperature` is the inverter's AC stage when a runtime message has given it, else the BMS MOSFETs from the display message.

**Conditional:** the same families define a `RuntimePropertyUpload` message with the electrical detail below. It is decoded whenever a device sends it, but no way to make a device push it over Bluetooth is known yet (a `ConfigWrite` of the `cfg_runtime_property_*_upload_period` fields is neither acknowledged nor acted on by a DELTA 3 Max Plus), so these variables should be expected to be absent.

| EcoFlow reading (runtime message)        | NUT variable                                          |
| ---------------------------------------- | ----------------------------------------------------- |
| AC input volts, amps                     | `input.voltage`, `input.current`                      |
| AC output volts, amps, frequency         | `output.voltage`, `output.current`, `output.frequency` |
| pack volts, amps, full capacity (Ah)     | `battery.voltage`, `battery.current`, `battery.capacity` |
| inverter AC stage temperature            | `ups.temperature` (preferred over the MOSFET reading) |
| 12 V rail volts, amps                    | `outlet.8.voltage`, `outlet.8.current`                |

## Command coverage

Current command coverage:

| Command / feature                     | Status          | Notes                                                                                      |
| ------------------------------------- | --------------- | ------------------------------------------------------------------------------------------ |
| `VER`                                 | implemented     | Returns daemon version string                                                              |
| `USERNAME`                            | implemented     | Simple username check                                                                      |
| `PASSWORD`                            | implemented     | Simple password check                                                                      |
| `LOGIN`                               | implemented     | Returns `OK` after auth                                                                    |
| `PRIMARY` / `MASTER`                  | implemented     | Returns `OK` after auth                                                                    |
| `FSD`                                 | implemented     | Returns `OK` after auth                                                                    |
| `LOGOUT` / `QUIT`                     | implemented     | Closes connection                                                                          |
| `PING`                                | implemented     | Returns `PONG`                                                                             |
| `LIST UPS`                            | implemented     | Basic `upsc` discovery                                                                     |
| `LIST VAR <ups>`                      | implemented     | Returns current variables                                                                  |
| `GET VAR <ups> <var>`                 | implemented     | Returns a single variable                                                                  |
| `GET UPSDESC <ups>`                   | implemented     | Returns configured description                                                             |
| `GET NUMLOGINS <ups>`                 | implemented     | Returns a fixed `1` today                                                                  |
| `GET TYPE / DESC`                     | not implemented |                                                                                            |
| `LIST CMD / RW / ENUM / RANGE`        | not implemented |                                                                                            |
| `SET VAR`                             | not implemented | Read-only daemon in this phase                                                             |
| `INSTCMD`                             | not implemented |                                                                                            |
| `STARTTLS`                            | not implemented |                                                                                            |
| `TRACKING`                            | not implemented |                                                                                            |
| strict NUT quoting/parsing edge cases | partial         | Good enough for current simple clients, not full protocol parity                           |
| full `upsmon` shutdown semantics      | not verified    | Command stubs exist, but end-to-end shutdown compatibility still needs hardware validation |

The discovery/read-only and authentication/session command groups are implemented. End-to-end `upsmon` shutdown compatibility remains unverified; administrative commands, TLS, and tracking remain incomplete.

NUT discovery and telemetry reads (`LIST UPS`, `LIST VAR`, `GET VAR`, `GET UPSDESC`, and `GET NUMLOGINS`), as well as `VER` and `PING`, do not require authentication. This allows standard `upsc` clients and the pfSense NUT status page to retrieve telemetry even when credentials are configured. `LOGIN`, `PRIMARY`/`MASTER`, and `FSD` still require authentication when credentials are configured. The web UI and API retain their existing authentication requirements.

Level 1: upsc compatible (discovery and read-only variables) [*Currently Implemented*]

- LIST UPS
- LIST VAR
- GET VAR
- GET UPSDESC

Level 2: authentication and session commands [*Implemented; shutdown compatibility unverified*]

- USERNAME / PASSWORD
- LOGIN
- GET NUMLOGINS
- PRIMARY / MASTER
- FSD
- LOGOUT

Level 3: admin/tool compatible (full read-only protocol) [*Yet to Implement*]

- GET TYPE / DESC
- LIST CMD / RW / ENUM / RANGE
- SET VAR
- INSTCMD

Level 4: full protocol (including TLS and tracking) [*Yet to Implement*]

- STARTTLS
- TRACKING
- strict quoting/parsing
- complete NUT error behaviour
