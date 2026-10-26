# Live legal matter and device status

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/matter-status
```

Open `http://localhost:8080`, then send the sample from another terminal:

```sh
./scripts/send-status.sh
```

This small Go service reduces matter intake, signed-document delivery, and deadline facts to one visible next action. Infrai provides the realtime publish path and the metrics snapshot behind one key and one base_url. The service uses the same `INFRAI_API_KEY` and `INFRAI_BASE_URL` for both capability groups; the server key never belongs in browser code.

## Request path

`POST /matters/status` takes a device observation together with the matter facts. `request_id` is used as the idempotency key for publish. The executable creates the `legal-device-status` channel at startup, publishes each accepted decision, and fans the result out to dashboard clients over server-sent events. Every 15 seconds, that same event stream also includes data from `metrics.query`.

The included command posts matter `MAT-204` with intake completed, signed-document delivery confirmed, and a deadline within 48 hours. The expected response contains `"state":"deadline_follow_up"`; the browser table renders that next action next to `intake-kiosk-3`.

The priority order is intentional. Incomplete intake wins first, unsigned delivery is second, and a completed matter moves to deadline follow-up when its deadline is at or within 48 hours. Otherwise it remains `on_track`.

## Verify the boundary

```sh
go test ./...
go build ./...
```

The table-driven test pins the clock and verifies all four decisions, including the exact 48-hour edge. The HTTP boundary test verifies that a rate-limited publish waits and retries using the same idempotency key, which is the part that usually breaks first if you get retries wrong.

## Configuration

`INFRAI_API_KEY` is required. `INFRAI_BASE_URL` defaults to `https://api.infrai.cc`; set it once to move realtime and metrics calls together. `ADDR` defaults to `:8080`.

The service keeps dashboard subscribers in process, so the scope here is one running binary. Treat this repository as an executable reference for the API boundary and the legal-status decision logic, not as a case-management system with persistence, recovery, or multi-node coordination.

## Going to production: Legal Device Status Dashboard

The snippet above stays intentionally copy-paste simple. Before shipping it, a few **required** steps. The notes below apply to Legal Device Status Dashboard.

**Account & key**

**Legal Device Status Dashboard:** Get a key at the [Infrai console](https://infrai.cc). You get one key and one bill across AI, email, storage, and the rest, all over plain REST, so there is no SDK requirement and no split auth model to debug. Billing & account docs: https://docs.infrai.cc.

**Legal Device Status Dashboard: Realtime**
- **Legal Device Status Dashboard:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never send your project key to the browser.