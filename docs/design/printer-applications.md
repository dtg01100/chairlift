# Printer Applications

<!--
Design docs are LIVING documents: update them in place so they always describe
the system as it is. Rationale does NOT live here — it lives in ADRs, linked
below. If you find yourself writing "because", consider whether it belongs in
an ADR.
-->

Living document. Rationale: [ADR-0016](../adr/0016-printer-app-admin-denied-until-authenticated.md).

This document covers ChairLift's support for the
[FSDK Printer Applications](https://github.com/projectbluefin/chairlift/issues/328):
rootless Podman quadlets, one unit per printer, for the Ghostscript,
PostScript, HPLIP, and Gutenprint driver families.

**Current state (2026-09-25):** the quadlet lifecycle
([#329](https://github.com/projectbluefin/chairlift/issues/329)) is in review
and not on `main`; ChairLift ships no printer UI today. Only the Ghostscript
family has a published image, and that image cannot yet receive the
administration settings ADR-0016 requires, so nothing can be enabled on a
LAN-facing surface. This page records the contracted network and access
surface so the lifecycle lands against a verified boundary instead of
inventing one.

## Overview

One enabled printer is one rootless unit in the invoking user's
`~/.config/containers/systemd`, driven with `systemctl --user` — the same
lifecycle as the local-AI stack, and equally off the pkexec path. Four driver
families exist; each family is one pinned image, and each printer within a
family is one app with its own unit, port, and state volume, so one logical
device has exactly one owner and one DNS-SD advertisement.

```
LAN client ── IPP ──► host :18010 ──► chairlift-printer-ghostscript (rootless quadlet)
                 ▲                       │ PAPPL: IPP + web interface on one listener
                 └── DNS-SD (host network)└─ web admin: denied until authenticated
```

## Network surface

The application runs on **host networking**. Two reasons, both verified:

- DNS-SD: the application advertises itself with the `avahi-daemon` it starts
  in-container. Under a bridge network that advertisement points at an address
  LAN clients cannot reach; under host networking it advertises the host.
- IPP: clients print to the application's port. Host networking puts PAPPL's
  listener on the host directly, so `PublishPort` is not used and cannot be
  mistaken for a boundary.

The consequence is that the web interface is on the same LAN-reachable port as
IPP — PAPPL serves both on one set of listeners and cannot bind administration
to loopback separately. The boundary is therefore authorization, not binding:
IPP-transport administration already refuses remote clients without an
authentication service, and web administration must be authenticated or
disabled before ChairLift enables the unit (ADR-0016). Today's Ghostscript
image forwards only `PORT` and a log file from its entrypoint, so it cannot be
configured to meet that condition — the image must change first.

## Family inventory

The contracted identities for the family-default app of each family. Distinct
container names, state volumes, and fixed non-colliding ports are the
enable-time acceptance for [#338](https://github.com/projectbluefin/chairlift/issues/338);
additional apps per family get their own non-colliding ports and volumes from
the same lifecycle.

| Family | Unit / container | Port | State volume |
| --- | --- | --- | --- |
| Ghostscript | `chairlift-printer-ghostscript` | 18010 | `~/printer-workspaces/ghostscript/ghostscript` |
| PostScript | `chairlift-printer-ps` | 18020 | `~/printer-workspaces/ps/ps` |
| HPLIP | `chairlift-printer-hplip` | 18030 | `~/printer-workspaces/hplip/hplip` |
| Gutenprint | `chairlift-printer-gutenprint` | 18050 | `~/printer-workspaces/gutenprint/gutenprint` |

Ports are above the privileged range and well below the ephemeral range; two
enabled families must not collide, and the lifecycle documents how a collision
is surfaced rather than silently retrying
([#329](https://github.com/projectbluefin/chairlift/issues/329)).

## Image state

Verified against GHCR on 2026-09-25:

- `ghcr.io/projectbluefin/ghostscript-printer-app:10.07.1-1` — published,
  multi-architecture OCI index (amd64 `sha256:9f647903…`, arm64
  `sha256:fef69f82…`). Runs as user 65532 with entrypoint
  `catatonit -- bash /usr/libexec/ghostscript-printer-app/container-entrypoint`.
- `projectbluefin/ps-printer-app`, `projectbluefin/hplip-printer-app`,
  `projectbluefin/gutenprint-printer-app` — no published GHCR repository
  (`tags/list` returns `NAME_UNKNOWN`).

The Ghostscript image's entrypoint honors one environment knob, `PORT` (passed
to PAPPL as `server-port`), and forwards no other options: extra arguments are
ignored, and there is no env path to `server-options`, `auth-service`, or
`admin-group` — the three settings pappl-retrofit itself supports. Until an
image publishes a way to receive those, enabling it on a LAN-facing surface
would expose unauthenticated web administration, which ADR-0016 forbids.

## Operational notes

- **Enable/disable is not pull/delete.** Disabling stops the service and
  removes the quadlet but leaves the pulled image and the per-app state volume
  untouched; rollback repoints a family at its prior verified index.
- **Unverified without hardware** (recorded as such, not claimed):
  - Actual printing through a physical device, and per-device passthrough
    scoping ([#330](https://github.com/projectbluefin/chairlift/issues/330)
    still owns device assignment).
  - The in-container `avahi-daemon` under host networking vs a host
    `avahi-daemon`: both compete for the mDNS socket; the collision behavior
    must be documented by the lifecycle, not discovered by a user.
  - Restart survival: the permitted IPP/DNS-SD surface must come back after
    `systemctl --user restart` with the same port and name, which the
    deterministic unit shape is designed for but has not been demonstrated
    against the real image.
- **Failure surfacing** ([#331](https://github.com/projectbluefin/chairlift/issues/331)):
  a missing image, a refused admin configuration, or a crashing service is an
  actionable, non-enabled state — never a false enabled indicator.

## References

- Rationale: [ADR-0016](../adr/0016-printer-app-admin-denied-until-authenticated.md)
- Related: [ADR-0001](../adr/0001-fixed-path-pkexec-privilege-boundary.md)
- Built in: lifecycle PR
  [#334](https://github.com/projectbluefin/chairlift/pull/334) (in review)
- Upstream: [pappl](https://github.com/michaelrsweet/pappl),
  [pappl-retrofit](https://github.com/OpenPrinting/pappl-retrofit),
  [ghostscript-printer-app](https://github.com/projectbluefin/ghostscript-printer-app)
