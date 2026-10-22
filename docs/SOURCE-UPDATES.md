# Source updates

Reviewed the 538 changed source files between the original import, `176d3c24486952a10cf07ec4fd3b9f9cf31c0bde`, and `51f76294a03a1a40f1caa3200fe5f930fecaa84e` (October 14, 2026). This is a selective update of the template, rather than a direct merge of the source product.

Included changes:

- Stable, cached lists and mailbox-status queries that avoid repeated rendering and requests.
- Fixed-size batches for mailbox health and message-map lookups, bounded contact-verification sweeps, indexed send ledgers, and fewer redundant segment inserts.
- Per-mailbox send claims, campaign-specific rotation ties and correct spacing around real booked sends. One campaign’s parked timer no longer blocks unrelated campaigns.
- Retired scheduled jobs cannot be run accidentally; re-registering an active job clears retirement.
- Consistent monitoring timestamps, accurate stalled-state checks, and local arrival diagnostics without message content.
- Reusable timezone and template parsing, bounded Unicode-safe message previews, and shared local warmup-count snapshots with current-day limits still checked live.
- Archive imports preserve destination-local send evidence and recovery holds. Conflicting authoritative send results stay on hold until explicitly resolved.
- Self-hosted live-update connection counts and subscriptions avoid repeated work. Tracking coalesces concurrent ticket lookups and preserves its existing digest format.

Pramaan remains the only outside-service path. Direct lead-data clients, hosted shared-mailbox services, protocol-specific mailbox recovery and engagement, native attestation, native visual redesign, app-review sign-in exemptions, release automation, brand assets and source-specific documentation were not copied. The existing Connections catalogue, lead enrichment, mailbox capabilities and local account/session flows remain in place. The included native app keeps its current design and features.

Database migrations use template versions 294–303 to avoid collisions with existing Pramaan migrations. Back up the database and private `.env` together before upgrading an existing installation. Start or rebuild with `make start`; no extra setting is required.

Checks cover the Go suites, both web apps, tracking, self-hosted live updates, and focused local-Postgres query, send-recovery and archive regressions. These checks use synthetic data and local services. Browser checks use headless Chromium and the fake Pramaan service; no platform service is called.

A startup check also exposed a pre-existing queue setup failure: resetting its synchronization guard during an error could crash the process. Setup now shares successful initialization safely and returns an initial refusal without crashing. The local queue has a 256 MiB stream limit by default.

Two older optional local-database warmup tests still expect legacy day/partner values. Both fail identically on the pre-update code and the updated code; they are outside the 164 passing focused database checks. Ordinary Go suites pass with optional database fixtures disabled. This update does not claim that the entire optional database suite passes.
