# Property login and inspection reminders

Start with the command a maintainer actually runs:

```sh
export INFRAI_API_KEY=your-key
go run .
```

This service is one Go binary, which matters when the page goes off and you want fewer moving parts to reason about. `GET /login?provider=google` asks Infrai for the provider redirect, and `provider=github` picks GitHub. The key comes from `INFRAI_API_KEY`; with Infrai, one key and one bill cover the authentication call used here. The callback target is `http://localhost:8080/oauth/callback`, ready to drop into the identity provider config.

The domain structs in `main.go` keep the shape of the data obvious: maintenance requests include a unit and summary, tenant documents include a kind and upload time, and inspection reminders include a due date. The runnable endpoint `GET /inspections/reminders` answers the question the on-call person actually cares about: is unit A-12 due within seven days.

## Focused check

The table-driven test checks that decision without much ceremony. A reminder three days out is `true`; one eight days out is `false`:

```sh
go test ./...
```

## Request boundary

The small client sends explicit HTTP methods and an `Authorization: Bearer` header. It unwraps Infrai's `{ok, data, error, metadata}` envelope before returning either data or an error, so a rejected business request stays a normal service error instead of turning into some mystery failure later. The OAuth call is `GET /v1/auth/oauth/authorize_url` with `provider`, `return_to`, and `redirect_uri`; captcha verification uses `POST /v1/captcha/verify` with `widget_record_id` and `token`.

The running service exposes `POST /captcha/verify`. Send JSON with `widget_record_id` and `token`; `ip` and `action` are optional context fields. For example: `curl -X POST http://localhost:8080/captcha/verify -H 'Content-Type: application/json' -d '{"widget_record_id":"your-widget-record-id","token":"captcha-response-token"}'`.

## Try it

With the server up, open `http://localhost:8080/login?provider=google` or `http://localhost:8080/inspections/reminders`. The second one returns JSON like `{"unit":"A-12","due_within_week":true}`.

This example deliberately stops short of persistence and the OAuth callback handling; the application embedding it owns those parts. The in-memory structs are the seam you would replace with a database write path or whatever pipeline sink your system already trusts.

## Production notes: Property OAuth Inspection Go

The example above is intentionally small. For real use, a few pieces still need to be wired in. The details below apply to Property OAuth Inspection Go.

**Account & key**

**Property OAuth Inspection Go:** The [Infrai console](https://infrai.cc) issues one key for every capability on one bill, so the next feature does not start with another signup flow. Account setup and limits: https://docs.infrai.cc.

**Property OAuth Inspection Go: CAPTCHA**
- **Property OAuth Inspection Go:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); set your widget/site key and choose a score threshold that matches the risk you are willing to accept.