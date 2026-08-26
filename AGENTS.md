# Development

This is a small, stateless Go webhook service. It acknowledges Text `chat_deactivated` webhooks, then requests and polls for the closed thread's summary in the background before creating or updating one HubSpot contact and its associated Note.

## Commands

```shell
gofmt -w cmd internal
go test ./...
go build ./cmd/contact-sync
docker compose up --build
docker compose down
```

## Constraints

- Keep external API code in `internal/textapi` and `internal/hubspot`.
- Keep the webhook handler limited to HTTP validation and response handling; orchestration belongs in `internal/contactsync`.
- The example intentionally has no database, queue, retries, or idempotency store. Production alternatives belong in `docs/edge-cases.md` unless the guide's scope changes.
- Do not add an SDK solely for Text or HubSpot. The direct HTTP calls are part of the guide.
