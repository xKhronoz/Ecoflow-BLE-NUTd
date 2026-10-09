# BLE authentication and device discovery

[Project overview](../README.md) · [Documentation index](README.md) · [Deployment options](../deploy/README.md)

You have two auth options for `eco-ble`:

- easiest: leave `provider.auth.user_id` empty and use `email/password`
- stricter/local after bootstrap: resolve your EcoFlow `user_id` once, then store that instead of the cloud password

Use the EcoFlow app account associated with the device. An EcoFlow Developer account and developer API access keys are not required for BLE authentication.

## Resolve the app account user ID

To obtain `user_id` directly:

1. Base64-encode your EcoFlow password:

   ```sh
   printf '%s' 'YOUR_PASSWORD' | base64
   ```

2. Call the same login API shape used by this daemon and extract `data.user.userId`:

   ```sh
   curl -s https://api.ecoflow.com/auth/login \
     -H 'Accept: application/json' \
     -H 'Content-Type: application/json' \
     -d '{
       "scene":"IOT_APP",
       "appVersion":"1.0.0",
       "password":"BASE64_PASSWORD_HERE",
       "oauth":{"bundleId":"com.ef.EcoFlow"},
       "userType":"ECOFLOW",
       "email":"you@example.com"
     }'
   ```

For accounts that need another region, change the host from `api.ecoflow.com` to the matching region such as `api-e.ecoflow.com` or `api-cn.ecoflow.com`. The returned JSON includes `data.user.userId`.

## Find the BLE MAC address

To obtain the BLE `mac` address:

1. On Linux with BlueZ, scan while the unit is awake:

   ```sh
   bluetoothctl
   scan on
   ```

   Look for an EcoFlow advertisement name and note the address shown next to it, for example `AA:BB:CC:DD:EE:FF`.

2. If scanning is noisy, match by the advertised device name or serial prefix, then cross-check the address in the EcoFlow app when available.

Use that address as `devices[].mac` in the config.

## Protocol cryptography

EcoFlow's BLE protocol requires MD5 for the type 7 IV and session-key derivation, the type 1 key and IV, and the user-ID authentication token. These operations match the [reference BLE implementation](https://github.com/rabits/ha-ef-ble/blob/main/custom_components/ef_ble/eflib/connection.py). Replacing MD5 with SHA-256 locally would change the bytes expected by the device and break authentication.

MD5 remains a weak cryptographic primitive. Its use here is a device-protocol constraint that maintainers must assess when reviewing CodeQL alerts; compatibility does not make the protocol cryptographically strong. The daemon does not use these functions as a password-storage mechanism.

## Authentication troubleshooting

If a build reports `illegal base64 data at input byte XX` during type X authentication, it may be that the build contains a malformed embedded key-data file from an earlier revision, or that the `user_id` or `password` in the config is not correct. Check that the `user_id` is a valid EcoFlow account ID and that the password is base64-encoded. If you are using a release binary, verify that it is a recent release and not an older build with known issues.

For unknown BLE MAC addresses, check that the device is awake and advertising, and that the host has a working BLE adapter such as `hci0`. Use `bluetoothctl` to scan for the device and confirm the address. If the device is asleep or out of range, it will not respond to authentication attempts.

For new errors, check the daemon logs with `sudo journalctl -u ecoflow-ble-nutd -f` or the console output if running in the foreground to see the authentication sequence and any error messages. Ensure that the configuration file is correct and that the daemon has access to the necessary BLE interface. Post any persistent issues to the project issue tracker or discussion forums with the relevant log excerpts and configuration details (excluding sensitive credentials).

If you are using a container, ensure that the container has access to the host's Bluetooth interface. You may need to run the container with the `--privileged` flag or mount the Bluetooth device into the container.

For container rebuild commands, see [Docker deployment](../deploy/docker/README.md#updating-and-stopping).
