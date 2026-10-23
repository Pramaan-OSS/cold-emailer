[![CI](https://github.com/Pramaan-OSS/cold-emailer/actions/workflows/ci.yml/badge.svg)](https://github.com/Pramaan-OSS/cold-emailer/actions/workflows/ci.yml)

# Cold Email

A self-hosted outreach workspace with local accounts, contacts, campaigns, inboxes and an assistant. Outside services use Pramaan through one Go client module; the web app uses `@pramaan/sdk` through its authenticated backend.

## Quick start

Install Docker with Compose and use `make` (included on most developer systems). No language runtime, package-registry account or other service account is needed.

```sh
cp .env.example .env  # enter only PRAMAAN_API_KEY in this file
make start
```

Open http://localhost:5173 (admin: http://localhost:5174). First startup builds the included code and SDK. The command starts local Postgres, Redis, NATS, the backend, consumer, worker and both web apps. It generates private session, encryption, internal-service and database keys in `.env`, preserving existing values. Back up `.env`, `.runtime` and the database volumes together. Never share or commit these private settings.

Run `docker compose -f docker-compose.dev.yml logs backend` to find the private first-run setup link. Create the owner account. The local mail transport writes any sign-in codes to those backend logs. Without a key, the app still starts and says “Add PRAMAAN_API_KEY to .env”; local features remain usable. Restart with `make start` after adding the key.

Stop with `docker compose -f docker-compose.dev.yml --profile fake down`; data stays in named volumes. The Rust tracking service, live-update gateway and public forms service are optional services in `docker-compose.yml`. The [deployment guide](deploy/README.md) and [split deployment guide](deploy/split-cloud/README.md) are optional deployment setups.

The only required setting is `PRAMAAN_API_KEY`. Optional settings are commented at the bottom of `.env.example`: the platform address, public app/API URLs, a hosting project, social sign-in, and connector/organisation callback overrides. `APP_URL` defaults to http://localhost:5173; `API_PUBLIC_URL` to http://localhost:8080. Connector return and sign-in callback URLs derive from these. Managed sign-in works on localhost by default; a public app address must use HTTPS. `PRAMAAN_FAKE=1` enables isolated synthetic checks with no platform traffic.

Sending domains offers “Find and buy a domain” with an owner/admin-only, explicit non-refundable price confirmation. Registration is currently switched off on the platform and has not been live-tested. Use “Use a domain you own” to attach an existing domain and follow the DNS records, zone file or setup link supplied by Pramaan. `PRAMAAN_HOSTING_PROJECT` is optional and only needed for the app's web-domain attachment.

Mailbox provisioning and mail-domain setup may currently answer “not available yet”; some company-data reads do too. The app shows that plainly and keeps local sign-in, contacts, campaign editing and settings usable. Owning a domain does not guarantee mail-domain attachment is currently available.

Notifications use the platform sender configured by `DefaultSenderDomain` in `internal/notify/email.go` through Pramaan when no sending account is connected. Set optional `EMAIL_ADDRESS` to the owner’s address to receive replies; it does not change the platform sender. A queued response confirms acceptance, not delivery. Reserved example and local recipient domains are refused before sending. Demo campaigns stay drafts and demo mailbox warmup is off.

Sign-in and the database are provided through Pramaan in test mode. First start provisions a managed sign-in backend and uses it for social sign-in, email sign-up and email sign-in; no separate social-client setup is needed. Its address is a sign-in server, and the app handles the browser flow and verifies the signed token through Pramaan before opening its own session. Local accounts, registration rules, account-link confirmation and two-factor checks remain in place. The private first-owner setup link is still required to claim a new instance. Local email/password sign-in remains available without a key or if managed sign-in is unavailable.

Startup also requests a managed database. If it is unavailable, the app prints one line and uses bundled local Postgres. A database that already exists does not return its credentials again: a fresh checkout without the saved private state stops for recovery instead of changing that database. When newly created, startup saves the one-time connection strings in `.runtime/managed.json` and `.runtime/managed.env` with private file permissions, then reuses them on later starts. Migrations use the direct connection and the app uses the pooled connection. Startup action references are also saved here and reused; a refused request stops startup with a plain reason. Keep this folder backed up: if the first response is lost or the credentials are missing, startup stops for manual recovery instead of silently creating or rotating another database. Connection strings are never printed.

Only public HTTPS app addresses are registered for managed sign-in. Localhost uses the sign-in backend's development allowance and is not sent for address registration. Captcha and commercial billing are off by default. Optional app-owned social credentials remain supported through `SOCIAL_PROVIDER_KEY`, `SOCIAL_CLIENT_ID`, `SOCIAL_AUTHORIZE_URL` and `SOCIAL_REDIRECT_URI`; they are not required for managed sign-in.
## Organisation sign-in

Organisation OIDC is implemented with fake tests only; the platform API is not deployed and has not been live-verified. Local account sign-in remains available. SAML, automatic member creation and custom scopes are not supported.

1. Set `API_PUBLIC_URL` to your public HTTPS API origin. Keep the web and API on the same site so the browser can retain the sign-in cookie. The callback defaults to `<API_PUBLIC_URL>/v1/auth/enterprise/callback`; `PRAMAAN_ENTERPRISE_REDIRECT_URI` can set an exact alternative. Register that callback with your organisation’s identity service.
2. An organisation owner or admin opens Settings → Security → Organisation sign-in. Enter the exact issuer, application client ID, public-client or secret redemption method, and any explicitly allowed endpoint hosts. Review and confirm creation. Client credentials go once to Pramaan; this app keeps only a connection reference.
3. Each existing member signs in locally, opens Security, checks saved sign-in connections and confirms **Link my organisation identity**. Linking uses this local session; email never finds or joins accounts.
4. Later, open **Organisation sign-in** on the login screen and enter the workspace and connection references from Security. Sign-in returns through Pramaan and creates the usual local session, with the same ban and second-factor checks. Connections are scoped to the organisation. An admin can confirm deletion without removing local users.

If sign-in or a connection change is unconfirmed, check it before another attempt; the app never retries a code exchange automatically. Keep local sign-in enabled so existing members can link their identities and recover from unavailable organisation sign-in.

## Development and checks

Go 1.26.9 and Node 26.12.0 with pnpm 10.26.2:

```sh
go test ./...
pnpm --dir web install --frozen-lockfile
pnpm --dir web typecheck
pnpm --dir web test:run --maxWorkers=4
pnpm --dir web build
```

The web and admin apps use the published `@pramaan/sdk@0.1.4` package. The web app’s single Pramaan module preserves long-running job waits and failed reports. Lead search can use the lead.find outcome: review its estimate before running; an unavailable outcome falls back to quoted company search. Tests use fake clients. `scripts/fake-pramaan.py`, `compose.local-check.yml` and `scripts/start-local-check.py` provide an isolated synthetic local check; they never call external services.

Without a Pramaan key, local sign-in, contacts, campaign editing, files and settings work; sending, enrichment, generation and connected services require a key. Shared warmup pools, raw mailbox protocol diagnostics, automatic directory delegation and the hosted challenge widget are unavailable. App actions and events depend on the connected catalogue; unavailable actions remain off. Local storage, Postgres, Redis, NATS and the self-hosted live-update service are retained infrastructure. No remote repository or deployment is created by this template.

## Optional billing

Checkout returns are checked against the saved workspace checkout. A return URL alone does not confirm payment or unlock a plan. Billing lifecycle-event processing still needs verification before paid-plan fulfilment can be enabled.

The one-command stack includes live updates through the API connection; it does not require another public port.

## Connected accounts

Connecting an outside account, such as a mailbox or another app, carries a monthly Pramaan charge for each connected
account, billed to the Pramaan account behind `PRAMAAN_API_KEY`. Signing in is free. Disconnecting an account ends its
charge.

## Built on

Built on [Warmbly / pirate-box](https://github.com/yogi100x/pirate-box), source commit `176d3c24486952a10cf07ec4fd3b9f9cf31c0bde`, originally Apache-2.0. Compatible fixes through source commit `51f76294a03a1a40f1caa3200fe5f930fecaa84e` (October 14, 2026) are adapted here; the [source update notes](docs/SOURCE-UPDATES.md) describe the selection. The original notice is preserved in `LICENSE-ORIGINAL`; frontend notices are preserved in their `NOTICE` files. This template is AGPL-3.0.
