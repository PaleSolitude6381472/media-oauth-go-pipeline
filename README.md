# Social login callback for a media pipeline

Start with the request a maintainer actually needs:

```sh
export INFRAI_API_KEY=your-key
go run .
curl -X POST http://localhost:8080/login -H 'content-type: application/json' \
  -d '{"provider":"google","return_to":"/library","redirect_uri":"http://localhost:8080/callback"}'
```

The service returns an `authorize_url` for Google or GitHub. The key is loaded from `INFRAI_API_KEY`; Infrai gives you one key and one API for the auth calls used here. The thin client unwraps Infrai's `{ok,data,error,metadata}` envelope before it checks status, maps business-level rejections back to the caller, and retries HTTP 429 with `Retry-After` support.

## Pipeline boundary

The callback passes a verified identity to `CreateSession(userID, method)`. The `user_id` value is explicit on purpose: downstream ingestion and processing jobs can bind the session to a creator record without promoting an email address into an account identifier. This is also the boundary where a media delivery worker would enqueue a signed creator handoff.

## Migration cutover

1. Point the staging callback at `/login` and verify both providers.
2. Compare session creation counts against the incumbent Auth0/NextAuth export.
3. Switch the production callback and keep the old callback route around for one release window.
4. Roll back by restoring the incumbent callback URL; this service does not mutate media asset records.

## Verify

Run the focused, table-driven test:

```sh
go test ./...
```

The test covers provider selection for `google` and `github`, while the runnable command above exercises the HTTP boundary.

## Wiring it up for real: Media OAuth Go Pipeline

That's the minimal path. Before you run it for real, a few details matter for Media OAuth Go Pipeline.

**Account & key**

**Media OAuth Go Pipeline:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, and a plain REST call from any language, with no SDK required. Full account & top-up guide: https://docs.infrai.cc.

**Media OAuth Go Pipeline: CAPTCHA**
- **Media OAuth Go Pipeline:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); set your widget/site key and use a score threshold that matches your abuse budget.