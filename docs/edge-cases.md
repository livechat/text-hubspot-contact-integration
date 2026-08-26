# Production Edge Cases

This repository favors a short, readable integration over production-grade delivery guarantees. The following concerns are intentionally outside its runtime scope.

| Problem | What the example does | Production options |
| --- | --- | --- |
| Duplicate or out-of-order webhook deliveries | It can create duplicate Notes because it stores no delivery or thread state. | Persist processed thread IDs or use an idempotency key and transactional outbox. Treat delivery as at-least-once. |
| Background sync fails after webhook acknowledgement | It returns `204` before the in-process worker completes, so the sync is lost on a process restart or provider failure. | Put a durable queue behind the acknowledgement, persist job state, retry with backoff, and alert on dead-lettered jobs. |
| Thread summary is generated asynchronously | It requests a summary after deactivation and polls the explicit thread for 60 seconds. A `409 already_exists` response and transient poll failures continue polling because an earlier delivery may have requested it or Text may be temporarily unavailable. | Request the summary in a durable worker, subscribe to `thread_summary_set`, and process the completed summary asynchronously. |
| Summary eligibility is not met | The Text API rejects the request or polling times out. | Validate that HIPAA is disabled, the plan supports summaries, and the thread includes a customer and public messages. For license-level automatic summaries, ensure there is a customer-authored public message and configure `chats.auto_summary_enabled` for newly deactivated chats. |
| Text or HubSpot is temporarily unavailable | One synchronous request fails the webhook. | Add bounded retries with backoff, a queue, a dead-letter workflow, and alerting. |
| HubSpot rate limits or transient errors | Errors are surfaced to the webhook caller. | Honor `Retry-After`, back off on `429`, `423`, and transient `5xx` responses, and cap concurrency per portal. |
| Contact lookup/create race | Two simultaneous deliveries for a new email can race. | Use HubSpot batch upsert where suitable, serialize by normalized email, or handle create conflicts by refetching. |
| Missing or changing customer email | A missing email skips the sync; an email change creates or finds a different contact. | Store a Text customer ID to HubSpot contact ID mapping, define identity migration rules, and provide reconciliation. |
| Multiple or invalid HubSpot contacts share an email | The example trusts HubSpot's lookup result. | Define duplicate-contact policy, use deduplication workflows, and surface ambiguous matches for review. |
| Text data should not overwrite CRM-owned data | The example updates non-empty Text name fields on every closed thread. | Define field ownership, update only on create, track source timestamps, or map Text data to dedicated custom properties. |
| International or multi-word names | The example splits the first whitespace-delimited word into `firstname`. | Keep a full-name custom property, adopt locale-aware parsing, or let users edit the CRM record. |
| Resumed chats and multiple threads | Every deactivated thread is considered independently. | Define whether one Note is needed per thread or per chat and persist the chosen identity. |
| Note formatting or size limits | The example escapes HTML but does not trim summaries. HubSpot Notes limit `hs_note_body` to 65,536 characters. | Truncate with an explicit marker, attach a transcript, or store a secure link to archived conversation data. |
| Timestamp accuracy | It uses `summary.updated_at`, falling back to receipt time. | Persist the deactivation event time, normalize timezone handling, and define the CRM activity-time policy. |
| Webhook authentication and replay protection | Text documents a shared `secret_key`, not a signed timestamped payload. | Terminate at an authenticated gateway, apply IP/network controls where available, add replay controls, and rotate secrets. |
| Public URL availability | Local tunnels and service restarts can make the configured URL unavailable. | Deploy behind stable HTTPS, health checks, monitoring, redundant instances, and a controlled configuration rollout. |
| PII, consent, retention, and deletion | The example copies customer identity and conversation summary to HubSpot. | Obtain legal approval, minimize fields, document retention, support erasure requests in both systems, and audit access. |
| Multi-portal HubSpot distribution | The example uses one private app token. | Implement OAuth authorization, encrypted token storage, refresh handling, tenant isolation, and installation lifecycle management. |
| Text webhook registration is repeated | Re-running the README command can create more webhook subscriptions. | Store webhook IDs, list existing configuration before registering, and unregister obsolete webhooks during deployment. |
| API changes | The example pins API paths known when written. | Monitor provider changelogs, contract-test provider payloads, and upgrade versions deliberately. |
