# Text Conversation Summaries to HubSpot

Creates or updates a HubSpot contact when a Text chat is deactivated, then requests the Text thread summary when needed and adds it as an associated HubSpot Note.

Full Text documentation: https://www.text.com/docs

## How It Works

```text
Text chat_deactivated webhook
  -> contact-sync webhook service
  -> immediate 204 response
  -> Text get_chat, request_thread_summary, and poll get_chat
  -> Text get_customer
  -> HubSpot find/create/update contact by email
  -> HubSpot Note associated with the contact
```

The service requires the Text customer email. It maps the Text name to HubSpot `firstname` and `lastname` by splitting at the first whitespace. It acknowledges a valid webhook immediately, then runs the sync in the background. When the initial explicit-thread lookup has no summary, it requests one for the closed thread and polls the same `thread_id` every two seconds for up to 60 seconds. Each poll request has a 10-second timeout, so a transient Text API timeout does not abort the remaining polls.

## Prerequisites

- Docker with Docker Compose.
- A publicly reachable HTTPS URL that forwards requests to local port `8080`. Use a tunnel during local development and deploy the container behind HTTPS for a persistent setup.
- A Text personal access token with:
  - `chats--all:rw`, or `chats--access:rw` when the token only needs access to its own groups
  - `customers:ro`
  - `webhooks.configuration:rw` for the webhook-registration command below
- A Text app client ID for `owner_client_id` when registering the webhook.
- A HubSpot private app token with:
  - `crm.objects.contacts.read`
  - `crm.objects.contacts.write`

## Quickstart

```shell
cp .env.example .env
```

Set the required values in `.env`.

`TEXT_BASIC_AUTH` is Base64-encoded `account_id:personal_access_token` value. Do not include the `Basic ` prefix. Never log this credential; revoke and replace it immediately if it is exposed.

> [!NOTE]
> You can configure PAT secret [here](https://www.text.com/app/settings/integrations/api-access/personal-access-tokens)

Generate `TEXT_WEBHOOK_SECRET` with a long random value, such as:

```shell
openssl rand -hex 32
```

Set `WEBHOOK_PUBLIC_URL` to the public HTTPS base URL for this service. Do not include a trailing slash.

Register the Text webhook. This command creates a license webhook for all chats visible to the token:

```shell
set -a
. ./.env
set +a

curl --fail-with-body \
  --request POST 'https://api.livechatinc.com/v3.6/configuration/action/register_webhook' \
  --header "Authorization: Basic ${TEXT_BASIC_AUTH}" \
  --header 'Content-Type: application/json' \
  --data "{
    \"action\": \"chat_deactivated\",
    \"url\": \"${WEBHOOK_PUBLIC_URL}/webhooks/text\",
    \"secret_key\": \"${TEXT_WEBHOOK_SECRET}\",
    \"owner_client_id\": \"${TEXT_OWNER_CLIENT_ID}\",
    \"type\": \"license\"
  }"
```


Start the service:

```shell
docker compose up -d --build
```

Confirm it is healthy:

```shell
curl --fail http://localhost:8080/healthz
```

## Summary Generation

`get_chat` only returns a summary already stored for the requested thread. On `chat_deactivated`, this service calls `request_thread_summary` if the summary is missing, then polls `get_chat` with the webhook's exact `chat_id` and `thread_id`. A completed summary has `status: "ok"`, `content.title`, and `content.summary_bullets`; the title and bullets become the HubSpot Note. A `409 already_exists` response means a previous delivery already requested the summary, so the service continues polling. Each poll request is limited to 10 seconds; failed polls continue until the 60-second summary deadline. The in-process background sync has a two-minute deadline and does not use the inbound webhook context, which prevents a provider timeout from canceling generation.

Text can generate a summary only for eligible threads. HIPAA must be disabled, the account plan must support summaries, and the closed thread must contain a customer and at least one public message. Automatic summaries also require a customer-authored message.

This flow does not require the `chats.auto_summary_enabled` license property because it explicitly requests each summary. That setting is useful when Text should create summaries without an API request; it applies only to newly deactivated eligible chats and requires separate Configuration API authorization.

The immediate acknowledgement intentionally favors a working local blueprint over durable delivery. If the service stops or a provider call fails after the `204`, the current sync is lost; see [docs/edge-cases.md](docs/edge-cases.md) for the production design.


## Configuration

| Variable | Required | Description |
| --- | --- | --- |
| `TEXT_BASIC_AUTH` | Yes | Base64-encoded `account_id:personal_access_token` for the Text APIs. |
| `TEXT_WEBHOOK_SECRET` | Yes | Shared secret checked on every webhook delivery. |
| `HUBSPOT_ACCESS_TOKEN` | Yes | HubSpot private app access token. |
| `TEXT_OWNER_CLIENT_ID` | Registration only | Client ID used by Text when registering the webhook. |
| `WEBHOOK_PUBLIC_URL` | Registration only | Public HTTPS base URL, without a trailing slash. |
| `HOST_PORT` | No | Docker host port; defaults to `8080`. |

## API Calls

The code intentionally uses small direct HTTP clients rather than provider SDKs:

- Text `get_chat` retrieves the closed thread and its stored summary, if any.
- Text `request_thread_summary` starts asynchronous generation for a closed thread, then the service polls the same explicit thread until it is stored.
- Text `get_customer` retrieves the customer's email and name.
- HubSpot reads a contact by email, creates it when absent, or updates non-empty name fields when found.
- HubSpot creates a Note with `hs_note_body` and `hs_timestamp`, associating it to the contact inline with association type `202`.

## Stopping And Rebuilding

```shell
docker compose down
docker compose up -d --build
```

The service keeps no local data. Rebuilding or restarting it does not affect contacts or Notes already created in HubSpot.

## Production Considerations

This is deliberately a minimal guide. See [docs/edge-cases.md](docs/edge-cases.md) before using this architecture in production.

## References

- [Text Agent Chat Web API](https://developers.livechat.com/docs/messaging/agent-chat-api/)
- [Text Request Thread Summary](https://developers.livechat.com/docs/messaging/agent-chat-api/#request-thread-summary)
- [Text Configuration API](https://developers.livechat.com/docs/management/configuration-api/)
- [Text Webhooks](https://developers.livechat.com/docs/management/webhooks/)
- [Text Personal Access Tokens](https://developers.livechat.com/docs/authorization/personal-access-tokens/)
- [HubSpot Contacts API](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/contacts/guide)
- [HubSpot Notes API](https://developers.hubspot.com/docs/api-reference/latest/crm/activities/notes/guide)
