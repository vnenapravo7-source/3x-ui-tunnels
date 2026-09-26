# OpenFlux, WDTT-Plus and CSQTT

This fork intentionally keeps these projects as sidecars instead of copying
their control planes into 3x-ui. 3x-ui owns the customer, expiry and
subscription; the dedicated service owns its dataplane and share-link payload.

## Add a configuration to a client

1. Create the user/configuration in the corresponding OpenFlux, WDTT-Plus or
   CSQTT server.
2. In 3x-ui open **Clients**, edit the client, choose **Links**, then select
   **Create tunnel link**.
3. Pick the format and enter the public endpoint plus that user's secret. The
   panel generates the official `openflux://v1/`, `wdtt://connect` or
   `csqtt://connect` payload and adds it to the client. You can still paste an
   existing link manually.

The link is then emitted byte-for-byte in the client's raw subscription and on
the HTML subscription page, where it has its own copy button and QR code. It is
not emitted in Xray JSON or Clash/Mihomo subscriptions because those clients do
not implement these protocols.

Native 3x-ui MTProto and AmneziaWG inbounds remain managed by 3x-ui and appear
in the same raw/HTML subscription as `tg://proxy` and `vpn://` entries.

## Operational boundary

- OpenFlux, WDTT-Plus and CSQTT binaries are not bundled or silently
  downloaded. The link builder creates the client import payload; it does not
  create the matching server-side account yet.
- CSQTT is licensed for noncommercial use unless you have a separate licence
  from its author.
- WDTT-Plus is GPL-3.0. If you redistribute a modified binary, comply with its
  source-code obligations.
- Treat every share link as a secret. It can contain passwords or keys.

Reusable server patches and deployment patterns are available in
[`SanityProtocol/swg-panel`](https://github.com/SanityProtocol/swg-panel). This
fork does not redistribute its WDTT/CSQTT builds.
