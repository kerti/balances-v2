# Project license: AGPL-3.0

**Status:** accepted

## Context

The repo has been public since alpha (ADR-0030) with no `LICENSE` file — technically "all rights
reserved," which undercuts the self-host pitch (ADR-0037/0039) that depends on people actually
being allowed to run and modify the code. A license needs picking before M7 productization work
leans on self-host being a real, legally-usable option.

The three live candidates were MIT/Apache-2.0 (permissive), AGPL-3.0 (network copyleft), and a
source-available scheme like BSL. The deciding constraints, from grilling:

- Self-hosting for personal/household use must stay unrestricted — that's the whole point.
- A closed-source competing hosted SaaS built on this code without giving anything back is not
  acceptable.
- An explicit patent grant is wanted (contributor sues over patents → loses their license).
- Whether to sell a commercial/enterprise tier later is undecided, but the option should stay
  open rather than be foreclosed by this choice.

## Decision

- **License: AGPL-3.0-only**, full text at repo-root `LICENSE`.
- **Self-hosting is unaffected.** AGPL's network clause (§13) triggers on offering the program to
  *other users* over a network; a household running its own instance for itself isn't distributing
  to third parties in the sense that matters here.
- **Network copyleft is the anti-strip-mining mechanism.** Anyone who forks this and runs it as a
  hosted service for others must offer those users the complete corresponding source, including
  their modifications. This doesn't ban a competing host outright, but removes the "take it,
  close it, sell it back" move — the exact scenario the permissive options don't guard against.
- **Patent grant comes for free.** §11 gives an explicit patent grant plus defensive termination,
  equivalent in spirit to Apache-2.0's — not a reason to prefer Apache over AGPL.
- **No SPDX header retrofit.** The root `LICENSE` file is legally sufficient on its own; adding
  `SPDX-License-Identifier` comments to every source file is pure hygiene with no bearing on
  validity. Deferred until a concrete reason shows up (tooling, multi-license mixing, external
  audit) rather than done speculatively.
- **No CLA this pass.** A Contributor License Agreement only matters once a second copyright
  holder exists. Solo-authored today, so relicensing later (e.g. dual-licensing, or moving to a
  more permissive license) needs no one's permission but the author's. Revisit the moment an
  external PR is about to merge, not before.
- **AGPL → permissive is the safe relicense direction, not the reverse.** Sole ownership means the
  *future* license can change at will (e.g. to Apache-2.0, if wider adoption ever outweighs the
  anti-strip-mining goal); already-released versions stay available under AGPL to whoever received
  them (can't be clawed back), but that's an acceptable one-way ratchet. Going the other direction
  (permissive → copyleft) would not be safe, since permissively-licensed code already in the wild
  can never be pulled back under new restrictions. This asymmetry is why AGPL was chosen as the
  starting point over Apache-2.0, keeping the option open rather than the reverse.

## Consequences

- `LICENSE` (AGPL-3.0 full text) lives at repo root, referenced from `README.md`.
- Contributor docs (if/when they exist) must add a CLA gate before the first external PR merges —
  otherwise a future relicense (dual-license or permissive-ization) is blocked without that
  contributor's explicit consent.
- No code or CI changes; this is a legal/docs-only decision.
