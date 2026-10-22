# Capability reference

All external operations go through the Go Pramaan client in `internal/pramaan`; the browser uses the published `@pramaan/sdk@0.1.0` through the authenticated backend. Request/result contracts come from the package exports.

Supported flows include mailbox and domain setup (`email.*`, `domains.*`), app discovery and connections (`apps.*`), remote tool connections (`connector.*`), lead search (`lead.find` with quoted `companies.search` fallback), generation (`llm.*`), storage (`storage.*`) and billing (`billing.*`, `payments.*`). Availability is checked at runtime.

Catalogue pages load on demand and are cached for the session. Connected file handles go directly to `document.parse`. App access is explained before hosted consent; templates cannot narrow that access yet.

Actions require the existing approval checks. An uncertain result must be checked before another attempt. Unavailable services show a clear message and preserve local editing and owned-domain setup.

Organisation OIDC uses `auth.enterprise-connection.create`, `auth.enterprise-connection.delete`, `auth.enterprise-connections.list`, `auth.enterprise.start` and `auth.social.exchange`. Administration requires an organisation owner/admin; linking requires an existing member’s local session. The platform API is not deployed and the flow has not been live-verified. Identity lookup uses a stable local connection ID plus subject, never email. State is checked against the initiating browser, redeemed once, and followed by the existing local session checks. SAML, custom scopes and automatic member creation are unavailable.
