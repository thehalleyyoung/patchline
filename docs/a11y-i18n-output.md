# Accessibility and i18n output

Patchline runs an **accessibility** and internationalization pass over all human-facing output, ensuring messages avoid color-only signaling and resolve through a message catalog.

## How it works

The worker checks each message has a non-color textual marker and a catalog key, and confirms the catalog covers every referenced key.

## What the gate proves

- All messages are accessible and fully localizable.
- A message relying on color alone with no catalog key is rejected.

## Why it matters

Accessible, localizable output widens the audience and is table stakes for a tool meant for global teams.

## Reproduce

```
make a11y-i18n-output-gate
```
