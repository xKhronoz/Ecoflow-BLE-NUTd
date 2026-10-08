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

## Authentication troubleshooting

If a build reports `illegal base64 data at input byte 99` during type 7 authentication, it contains a malformed embedded key-data file from an earlier revision. Update the source, rebuild the binary or Docker image, and restart the service or recreate the container using that build. The key table is embedded at compile time, so editing a file inside an existing container will not update the running binary.

DELTA 3 uses the supported type 7 handshake. Changing `devices[].model` does not change authentication or protocol selection; that field is display metadata. After rebuilding, check for `eco-ble authenticated` and fresh telemetry. A different authentication error should be investigated separately.

For container rebuild commands, see [Docker deployment](../deploy/docker/README.md#updating-and-stopping).
