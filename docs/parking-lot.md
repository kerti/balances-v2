# Parking lot

Deferred-but-not-forgotten ideas — things we consciously scoped *out* of a feature
so we wouldn't lose them. One line each; promote to a tracked issue when it's time.
This is a brain dump, not a contract.

## Backup & restore (ADR-0036)

- **Operator CLI restore** — `balances restore backup.json` loading into an empty DB with
  no prior sign-in, sidestepping the in-app bootstrap-then-wipe dance for the cleanest
  disaster-recovery path. In-app restore already serves both SaaS↔self-host directions, so
  this is a convenience, not a blocker.
- **Deleting / restoring household members** — a user-lifecycle Recycle Bin (soft-delete a
  member, resurrect them) within a household. Distinct from backup; backup carries
  soft-deleted users verbatim but never resurrects them.
- **Standalone "delete my account / delete household" UI** — a user-initiated wipe as its own
  surface. The wipe *mechanism* ships inside restore (wipe-then-load), but exposing it as a
  standalone destructive button is a separate feature.
- **Async restore (background job)** — only if real restores ever get slow enough that a
  synchronous request is a poor fit. No job system exists today; streaming keeps restore
  bounded, so this stays parked until measured need.
- **Editable Household display name** — today it's auto-derived from the founder's OAuth claim
  (*"{Name}'s Household"*) and never surfaced or edited in the UI. The restore summary *displays* it
  for context, which is adequate; letting the Household rename itself is a separate small feature.
