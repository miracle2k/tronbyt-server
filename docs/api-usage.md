# API Usage

This server exposes device APIs under:

```
$SERVER/v0/devices/$DEVICE_ID/...
```

All device endpoints require the device API key:

```
Authorization: Bearer $API_KEY
```

## Notifications

Notifications are high-level, short-lived messages. They can be created from:
- A base64 WebP image (`image`), or
- Text fields (`title`, optional `subtitle` and `subtitle2`), which the server renders.

At least one of `pinForSec` or `interstitialForSec` must be > 0.

### Create (text notification)
```bash
curl -sS -X POST "$SERVER/v0/devices/$DEVICE_ID/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "title":"Dishwasher done",
    "subtitle":"Kitchen",
    "subtitle2":"Please unload",
    "pinForSec":300,
    "interstitialForSec":900,
    "source":"homeassistant",
    "key":"dishwasher_done"
  }'
```

### Create (image notification)
```bash
curl -sS -X POST "$SERVER/v0/devices/$DEVICE_ID/notifications" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "image":"<base64-webp>",
    "pinForSec":300,
    "interstitialForSec":900,
    "source":"homeassistant",
    "key":"dishwasher_done"
  }'
```

### List
```bash
curl -sS -H "Authorization: Bearer $API_KEY" \
  "$SERVER/v0/devices/$DEVICE_ID/notifications"
```

### Delete by ID
```bash
curl -sS -X DELETE -H "Authorization: Bearer $API_KEY" \
  "$SERVER/v0/devices/$DEVICE_ID/notifications/$NOTIF_ID"
```

### Delete by dedupe key
```bash
curl -sS -X DELETE -H "Authorization: Bearer $API_KEY" \
  "$SERVER/v0/devices/$DEVICE_ID/notifications/by-key?source=homeassistant&key=dishwasher_done"
```

Notes:
- `source` + `key` is optional but recommended. It lets you update/delete without tracking IDs.
- `priority` is optional; higher wins if multiple notifications overlap.
- During night mode, only overrides with priority >= 100 are eligible.

## Overrides

Overrides are low-level, API-driven images that temporarily take over display.
Kinds:
- `pinned`: show ASAP after the current app finishes, for a duration.
- `interstitial`: show between apps for a duration, optionally less frequently via `everyN`.

### Create pinned override
```bash
curl -sS -X POST "$SERVER/v0/devices/$DEVICE_ID/overrides" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "kind":"pinned",
    "durationSec":300,
    "displayTimeSec":10,
    "priority":0,
    "image":"<base64-webp>"
  }'
```

### Create interstitial override
```bash
curl -sS -X POST "$SERVER/v0/devices/$DEVICE_ID/overrides" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "kind":"interstitial",
    "durationSec":1200,
    "displayTimeSec":10,
    "everyN":1,
    "priority":0,
    "image":"<base64-webp>"
  }'
```

### List / delete
```bash
curl -sS -H "Authorization: Bearer $API_KEY" \
  "$SERVER/v0/devices/$DEVICE_ID/overrides"

curl -sS -X DELETE -H "Authorization: Bearer $API_KEY" \
  "$SERVER/v0/devices/$DEVICE_ID/overrides/$OVERRIDE_ID"
```

Notes:
- `displayTimeSec` controls dwell time for the override (optional; defaults to device interval).
- `everyN` is only valid for interstitial overrides; `1` means every gap.

## Base64 WebP helper

If you want a quick test image:

```bash
magick -size 64x32 xc:black -fill white -pointsize 12 -gravity center \
  -annotate 0 "TEST" /tmp/test.webp
base64 /tmp/test.webp | tr -d '\n'
```
