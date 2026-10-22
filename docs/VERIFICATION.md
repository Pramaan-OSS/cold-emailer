# Verification

Tests use fake clients and synthetic records. The Go suite, web and admin tests and production web builds cover local behavior. Browser checks use headless Playwright and close their browser sessions. Logs and screenshots are local evidence and are not shipped.

No live delivery, registration, connected write actions, payment or remote tool calls have been verified. Native mobile builds are not covered. Use one project per organisation for any owner-run live checks until the platform removes the first-use limitation.

Outcome runs require estimates and approval, retain their input and plan, and never automatically resubmit. Capacity refusals mean try later. Long runs can be cancelled; the platform fails runs still open after 35 minutes. Unknown step effects mean check before retrying.

Organisation OIDC is checked only with synthetic hosted pages and fake Pramaan identities: approved connection creation, explicit member linking, returning local sessions, known-identity sign-in, wrong-state/replay refusal and confirmed deletion. No live identity service is contacted. The platform API is not deployed. SAML remains unavailable.

Generation field definitions are checked against the bounded JSON Schema subset before sending. The checks cover all fixed workspace, campaign, automation, research and warmup schemas, plus discovered remote definitions. Refused constraints name their field path; unsupported remote definitions are unavailable to the assistant, while their direct approved calls remain separate. The local fixture uses the production client's strict check. Model requests use messages and an output budget of at least 64 tokens.

Mailbox/domain setup, domain registration and some company-data calls may currently answer not available yet. Fake tests verify plain refusal messages, no completed result, no repeated action and continued use of the rest of Connections. Failed model requests and actions stay failed with a reason; fresh model retries are explicit, and uncertain action results require checking before retrying. Headless checks use synthetic data only.

Cut-off replies are incomplete, never completed work. Free-text writing can continue once within a bounded budget; structured replies and action requests fail before parsing or execution. Streamed partial text remains visibly incomplete. An empty reply refused because its output limit is too small requires increasing that limit; it is not automatically retried. Application defaults use at least 256 tokens, and the hard minimum is 64. Paid partial replies retain their charge, while an uncharged refusal remains separate.
