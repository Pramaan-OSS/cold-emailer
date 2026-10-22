# ColdEmail iOS

Native SwiftUI companion app for the ColdEmail dashboard. Covers the mobile-relevant dashboard surface: unified inbox (read, reply, snooze), mailbox accounts with warmup monitoring and controls, campaign monitoring with start/stop, contacts with the 360 view, analytics, deliverability, CRM (deals, tasks, meetings), templates, team management, notifications, and profile/billing views. Realtime by default: the app joins the Phoenix websocket and refreshes lists live off the same `AUDIT_CREATED` spine the web uses, including org presence (who is viewing/editing which record).

## Requirements

- Xcode 26 or newer (the project uses the folder-synchronized project format and an iOS 26 deployment target).
- A running backend. For local dev: `make infra`, `make backend`, and `make realtime` from the repo root (the realtime service powers live updates and presence; the app degrades gracefully without it).

## Running

1. Open `ios/ColdEmail.xcodeproj` in Xcode.
2. Select the ColdEmail scheme and an iOS 26 simulator, then run.
3. On the login screen, tap the gear to check the connection settings:
   - Server origin: `http://localhost:8080` by default (the app calls `<origin>/v1`). On a physical device, use your Mac's LAN address.
4. Sign in with a seeded user (`make seed` creates `dev@example.test` / `password123`) or register from the app. Password login is two-step: credentials, then an emailed 6-digit code (visible in Mailpit at `http://localhost:18025` during local dev).

For a full local development walkthrough (running the stack, the first-run flow, physical devices, and troubleshooting), see [DEVELOPMENT.md](DEVELOPMENT.md).

## Architecture

- `ColdEmail/Core/Networking/APIClient.swift`: async client for the `/v1` API. Bearer auth, single-flight token refresh (refresh tokens are single-use server-side), one retry on 401, `Idempotency-Key` on mutating calls that need it.
- `ColdEmail/Core/Session/SessionStore.swift`: login (password + OTP + optional TOTP), registration, org selection. The active org is server-side session state, so the store re-POSTs `/organization/switch/:id` once per launch, mirroring the web's OrgGate.
- `ColdEmail/Core/Realtime/`: a Phoenix Channels client speaking the V1 object-frame serializer (`vsn=1.0.0`) over `URLSessionWebSocketTask`, with 25s heartbeats, an 8s reply watchdog, resume via `last_seq`, and jittered reconnect backoff. `RealtimeService` maps events onto invalidation "pulses" that views observe, and tracks org presence.
- `ColdEmail/DesignSystem/`: the brand translated to native iOS. Slate neutrals with sky as the single accent, status tones matching the web (emerald running, amber paused, rose issues), eyebrow labels, thin tabular stat numbers, presence avatars with amber/emerald rings.
- `ColdEmail/Features/`: one directory per feature; each screen loads with `.task`, reloads on realtime pulses, and supports pull-to-refresh.

The Xcode project uses `PBXFileSystemSynchronizedRootGroup`, so new Swift files under `ColdEmail/` are picked up automatically; the pbxproj never needs editing for added files.

## Known limits

- Mailbox connect (OAuth/SMTP) and campaign sequence editing stay on the web; the app links out and says so.
- Native social sign-in is unavailable; use local account sign-in.
- Light appearance first, matching the production web dashboard; colors are defined as dynamic pairs so a dark palette can land later.

Captcha is off by default; no hosted browser challenge is available. Optional server tokens use the neutral bot-check contract.
