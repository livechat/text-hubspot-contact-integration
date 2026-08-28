# Text Conversation Summaries to HubSpot

Creates or updates a HubSpot contact when a Text conversation closes, then adds the Text-generated conversation summary to the contact timeline.

Full setup guide: https://www.text.com/docs/guides/sync-hubspot-crm

## Quickstart

Requires a Text personal access token with the `chats--all:rw` or `chats--access:rw`, `customers:ro`, and `webhooks.configuration:rw` scopes, and a HubSpot private app token with the `crm.objects.contacts.read`, `crm.objects.contacts.write`, and `crm.objects.notes.write` scopes.

Follow the setup guide above to create the credentials, expose the webhook endpoint, and register the Text webhook.

```shell
git clone https://github.com/livechat/text-hubspot-contact-integration.git
cd text-hubspot-contact-integration
cp .env.example .env    # set TEXT_BASIC_AUTH, TEXT_WEBHOOK_SECRET, and HUBSPOT_ACCESS_TOKEN

docker compose up -d --build
curl --fail --include http://localhost:8080/healthz
```

A healthy service returns HTTP `204`.

## Configuration

| Variable               | Used by                          | Description                                                             |
| ---------------------- | -------------------------------- | ----------------------------------------------------------------------- |
| `TEXT_BASIC_AUTH`      | Service and webhook registration | Base64-encoded `account_id:personal_access_token` credential from Text. |
| `TEXT_WEBHOOK_SECRET`  | Service and webhook registration | Shared secret checked on every Text webhook delivery.                   |
| `HUBSPOT_ACCESS_TOKEN` | Service                          | HubSpot private app access token.                                       |
| `TEXT_OWNER_CLIENT_ID` | Webhook registration             | Text OAuth client ID that owns the webhook.                             |
| `WEBHOOK_PUBLIC_URL`   | Webhook registration             | Public HTTPS address for the service, without a trailing slash.         |
| `HOST_PORT`            | Service                          | Local port mapped to the service container. Defaults to `8080`.         |

---

See the full [Text documentation reference](https://www.text.com/docs).
