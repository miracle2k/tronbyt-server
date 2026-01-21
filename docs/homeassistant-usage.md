# Home Assistant Usage

This guide shows a simple REST-based setup that does not require a custom add-on.
It uses Home Assistant's built-in `rest_command` and optional scripts.

## 1) Store secrets

In `secrets.yaml`:

```yaml
tronbyt_notify_url: "https://YOUR_SERVER/v0/devices/YOUR_DEVICE_ID/notifications"
tronbyt_auth: "Bearer YOUR_DEVICE_API_KEY"
```

## 2) Add a reusable rest_command

In `configuration.yaml`:

```yaml
rest_command:
  tronbyt_notify:
    url: !secret tronbyt_notify_url
    method: POST
    headers:
      Authorization: !secret tronbyt_auth
      Content-Type: "application/json"
    payload: >
      {"title":"{{ title }}",
       "subtitle":"{{ subtitle }}",
       "subtitle2":"{{ subtitle2 }}",
       "mode":"{{ mode | default('expiring') }}",
       "pinForSec":{{ pin_for_sec|default(300) }},
       "interstitialForSec":{{ interstitial_for_sec|default(900) }},
       {% if (mode | default('expiring')) == "interstitial-sticky" %}
       "interstitialEveryN":{{ interstitial_every_n|default(1) }},
       {% endif %}
       "source":"homeassistant","key":"{{ key }}"}
```

Restart Home Assistant after editing `configuration.yaml`.

## 3) Test it in the UI

Home Assistant UI recently renamed Services to Actions:

- Developer Tools -> Actions
- Choose `rest_command.tronbyt_notify`
- Paste data:

```yaml
title: "Dishwasher done"
subtitle: "Kitchen"
subtitle2: "Please unload"
key: "dishwasher_done"
pin_for_sec: 300
interstitial_for_sec: 900
```

### Persistent pinned notification

```yaml
title: "Dishwasher done"
subtitle: "Kitchen"
key: "dishwasher_done"
mode: "pin-sticky"
pin_for_sec: 0
interstitial_for_sec: 0
```

### Persistent interstitial notification

```yaml
title: "Laundry done"
key: "laundry_done"
mode: "interstitial-sticky"
interstitial_every_n: 2
pin_for_sec: 0
interstitial_for_sec: 0
```

Note: The payload defaults to `mode: expiring`. For sticky modes, keep `pin_for_sec` and `interstitial_for_sec` at `0` so the API doesn't receive durations.

## 4) Use it in an automation

```yaml
action:
  - service: rest_command.tronbyt_notify
    data:
      title: "Dishwasher done"
      subtitle: "Kitchen"
      subtitle2: "Please unload"
      key: "dishwasher_done"
```

## Optional: wrap it in a script (macro)

Scripts can accept fields so you do not repeat the same data everywhere:

```yaml
script:
  tronbyt_notify:
    alias: Tronbyt notify
    fields:
      title:
      subtitle:
      subtitle2:
      mode:
      key:
      pin_for_sec:
      interstitial_for_sec:
      interstitial_every_n:
    sequence:
      - service: rest_command.tronbyt_notify
        data:
          title: "{{ title }}"
          subtitle: "{{ subtitle }}"
          subtitle2: "{{ subtitle2 }}"
          mode: "{{ mode | default('expiring') }}"
          key: "{{ key }}"
          pin_for_sec: "{{ pin_for_sec | default(300) }}"
          interstitial_for_sec: "{{ interstitial_for_sec | default(900) }}"
          interstitial_every_n: "{{ interstitial_every_n | default(1) }}"
```

Then call it from an automation:

```yaml
action:
  - service: script.tronbyt_notify
    data:
      title: "Dishwasher done"
      subtitle: "Kitchen"
      subtitle2: "Please unload"
      key: "dishwasher_done"
```
