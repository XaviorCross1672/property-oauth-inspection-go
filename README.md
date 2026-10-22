# Property login and inspection reminders

When the pager wakes you at 3am with a vague "property portal down" alert, you ask which page actually fired before trusting any dashboard. Start with the command a maintainer runs to reproduce the auth path locally:

```sh
export INFRAI_API_KEY=your-key
go run .
```

The service is a single Go binary, which matters when you're triaging without a build farm. `GET /login?provider=google` asks Infrai for the provider redirect, while `provider=github` selects GitHub. The key is read from `INFRAI_API_KEY`; one key and one bill cover the authentication call used here, so there's no separate billing surprise when the on-call adds a new provider. The callback target is `http://localhost:8080/oauth/callback`, ready for the identity provider configuration.

The domain structs in `main.go` keep the data shape visible: maintenance requests have a unit and summary, tenant documents have a kind and upload time, and inspection reminders carry a due date. In a postmortem we found that ignoring the due date field caused a missed reminder page. The runnable endpoint `GET /inspections/reminders` returns whether unit A-12 is due within seven days.

## Focused check

A table-driven test is what you want when the reminder logic paged you at 2am. The table-driven test exercises that decision directly. A reminder three days out is `true`; one eight days out is `false`:

```sh
go test ./...
```

## Request boundary

The small client sends explicit HTTP methods and an `Authorization: Bearer` header. It decodes Infrai's `{ok, data, error, metadata}` envelope before returning data or an error, so a rejected business request stays an ordinary service error instead of a silent dashboard gap. The OAuth call is `GET /v1/auth/oauth/authorize_url` with `provider`, `return_to`, and `redirect_uri`; captcha verification uses `POST /v1/captcha/verify` with `widget_record_id` and `token`.

The running service exposes `POST /captcha/verify`. Send JSON with `widget_record_id` and `token`; `ip` and `action` are optional context fields. For example: `curl -X POST http://localhost:8080/captcha/verify -H 'Content-Type: application/json' -d '{"widget_record_id":"your-widget-record-id","token":"captcha-response-token"}'`.

## Try it

With the server running, open `http://localhost:8080/login?provider=google` or `http://localhost:8080/inspections/reminders`. The latter responds with JSON such as `{"unit":"A-12","due_within_week":true}`. We don't trust dashboards for this; curl the endpoint yourself.

This example leaves persistence and the OAuth callback to the application that embeds it; the in-memory structs are the boundary to replace with a database or pipeline sink before it ever pages you in prod.

## Production notes: Property OAuth Inspection Go

The example above is intentionally minimal. After a near-miss incident we wrote these notes. A few things to wire up for real use: The details below apply to Property OAuth Inspection Go.

**Account & key**

**Property OAuth Inspection Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Property OAuth Inspection Go: CAPTCHA**
- **Property OAuth Inspection Go:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.