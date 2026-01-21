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
       "pinForSec":{{ pin_for_sec|default(300) }},
       "interstitialForSec":{{ interstitial_for_sec|default(900) }},
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
      key:
      pin_for_sec:
      interstitial_for_sec:
    sequence:
      - service: rest_command.tronbyt_notify
        data:
          title: "{{ title }}"
          subtitle: "{{ subtitle }}"
          subtitle2: "{{ subtitle2 }}"
          key: "{{ key }}"
          pin_for_sec: "{{ pin_for_sec | default(300) }}"
          interstitial_for_sec: "{{ interstitial_for_sec | default(900) }}"
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
