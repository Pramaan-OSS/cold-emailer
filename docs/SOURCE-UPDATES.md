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

## October 25, 2026 — compatible fixes through October 24

Reviewed source range `51f76294a03a1a40f1caa3200fe5f930fecaa84e..ec3e1df78827d5ff51015394be3a16f8ece8dab0`. This remains a selective adaptation, without a source merge or new outside transport.

Included by area (source commit prefixes):

- Campaign content: bounded functions, range execution, variables, arguments and output (`1a77dbbd2`); HTML context escaping in sends, placement and previews, with trusted mailbox signatures preserved (`54e6910bb`). Template-specific generation and integration adapters remain in place.
- Decryption: uniform errors for invalid sealed data and refusal before any Pramaan send when message content or its workspace key cannot be opened (`60cef7bcd`). No key-rotation machinery was introduced.
- Request/redirect safety: bounded strict XML without directives (`976c99b4a`); repeated single-value query parameters and credential headers refused, forwarding headers normalized (`9bb0a39e9`); form and sending-domain redirects checked against workspace-owned domains at save and serve time (`28277cad5`); links in non-development log mail redacted (`f2f42792f`). Existing federated sign-in state already uses atomic consume; held local identity-link challenges now also consume once before issuing a session, adapting the one-use principle of `107bd854c` without its removed sign-in client.
- Scheduling and sends: calendar-date stepping through daylight-saving changes (`8e419fc2c`, `913dff65d`), preserved nanoseconds within a preferred send hour (`896d12b8c`), refused manual sends when ban or suspension reads fail (`e29593501`), organization-wide newest-reply lookup (`fdc611c80`) and sender-header address parsing (`dbfb46614`).
- Warmup and recovery: stopped participation retires queued work under the same locks as dispatch (`e8597e8af`); paused or never-started mailboxes cannot receive new warmup mail (`12c87181e`, `c93c88dd3`); admin send-hold listing and permission-checked resolution, including `task_missing`, plus narrower failure counts (`98e4b1014`, `c7247e4da`). Existing confirmed-send evidence rules are preserved. Queries use the template schema without the source-only retired-send column; time comparisons carry explicit database timestamp types. Explicit warmup stops use the database clock, so a small database/application clock difference cannot leave a newly queued command alive. Protocol recovery fixtures now supply an actively participating recipient.
- Sessions/team: absolute session expiry and capped refreshed tokens (`208ed1e34`), revoke-before-write password changes and optional app/key revocation, body-carried password-reset exchange (`e0a97ef52`), 48-hour invitations with expired listings/resend (`0d76f81a3`) and transactional invitation consumption (`6bb947d4b`).
- Clear refusals: stable codes for session, password, invitation, sending restrictions, repeated input and redirects (`9be5c8a3c`), JSON realtime handshake errors and admin reconnection guidance (`cc1dc2f64`). Existing session-bound, one-use realtime tickets, scoped credentials and second-factor admin joins remain in place.

Skipped: direct SMTP/IMAP resolved-peer dialing, TLS AUTH and certificate pinning changes (`d7bacf611`, `4624cc4a7`, `573e79ce8`, `920013807`, associated connection error screens) because mailbox network operations are owned by Pramaan and the template contains no active direct mail dial path. The retained autoconfig parser has no network fetch and is still bounded. Hosted pool links, mailbox adoption, whole-domain delegation and tenant connections remain removed or gaps; no direct vendor client was restored. AWS/Terraform/deployment, key rotation, release machinery, original branding/docs/skills and unrelated product changes were excluded. No live calls or pushes.

Migration mapping: source `000300_warmup_stop_evidence` → template **000305_warmup_stop_evidence**, after existing template migration 304. Source protocol certificate-pin migrations are not applicable. Back up the database and private configuration together before upgrade; run with `make start` as before.

The existing profile-gap integration check now records and compares the previous send at whole-second precision, avoiding false failures from elapsed query time or discarded fractional seconds at its exact minimum boundary; 100 consecutive database runs pass. A headless browser check also found that approved outcome requests lacked the browser header allowance for `Pramaan-Plan`. The API now permits this approval header from allowed app origins, with an actual preflight regression check. Validation passed locally under UTC on the committed `git archive HEAD` copy in `.scratch/sync/archive`, installed with frozen lockfiles and without copied dependencies. The final archive content was compared to the committed files. Tests use the fake client, an empty platform key and dedicated local Postgres/Redis; browser fixtures use a synthetic key against a loopback fake service. No live platform calls were made. This is local execution of the workflow commands, not a new hosted CI run.

- Go: module download, formatting, migration-pair/version guard, `go vet ./...`, `go build ./cmd/...`, and the full `go test -p 1 -json ./... -count=1` command passed **5,922 test cases including subtests**. The only skipped test is `TestRecordFixtures`, which requires live paid responses. New real-database tests cover send holds without a task, scoped/address-parsed replies, transactional invitations, stopped/paused recipients, dispatch retirement and recovery. The query preparation sweep passes against the actual template schema.
- Python: the workflow's unittest discovery and local configuration checks passed.
- Frontends: frozen installs for web/admin/forms/site/qa; web/admin/forms lint; web/admin/forms/qa type checks; **1,409 web and 127 admin tests**; web/admin/forms/site production builds all passed. Existing lint/compiler warnings remain.
- Tracking: pinned Rust 1.96.0; locked format/check/tests including ignored/build passed, **62 tests, zero ignored**.
- Realtime: workflow-pinned Elixir image; locked dependency checks, formatting, **133 tests** and production compilation passed.
- Headless browser: local sign-in, fake mailbox connection and explicit read test, and campaign preview passed. Browser requests were confined to localhost; zero outside requests. No email was sent.

The test-only `cold-email-sync*` Docker stack and local app/browser processes were removed after verification. No image was built, no shared stack was changed, and nothing was pushed.
