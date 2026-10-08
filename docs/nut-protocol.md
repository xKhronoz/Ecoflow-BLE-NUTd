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
