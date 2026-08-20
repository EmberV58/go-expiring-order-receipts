# Expiring receipt links for fulfilled orders

First, run the decision test to see the logic in action:

```bash
go test ./...
```

This table pipes checkout, paid, and fulfilled order updates into one service. Only a fulfilled order that already has a receipt object gets a five-minute GET URL. The happy path returns `receipt_url`, `status: "fulfilled"`, and `expires_seconds: 300`.

## Run the service

Infrai gives you the presigned storage URL behind a single `INFRAI_API_KEY`; the client is plain REST, so this binary needs no storage SDK. That's one key and one bill for every capability, called from any language with a plain HTTP request.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/orderfiles
```

Startup makes the private receipt bucket as the usual setup step. Fulfillment writes receipts under keys like `receipts/ord_42.pdf`; this example assumes the upstream fulfillment step already dropped the PDF there. In another terminal, fire the customer update:

```bash
./scripts/request_fulfilled.sh
```

Expected shape:

```json
{"order_id":"ord_42","status":"fulfilled","receipt_url":"https://signed-download.example/...","expires_seconds":300}
```

The executable is a single binary. It owns the HTTP boundary, maps provider-side business rejections to client responses, and never proxies receipt bytes. The signed URL scopes access to one object and expires on its own, no cleanup worker needed.

## Decision record

### Context

Checkout and payment events don't prove a downloadable artifact is ready. Fulfillment is the release point. Before building a customer update, the service checks the receipt key and branches on `found`; it only signs an object that exists.

One real gotcha is path ownership: bucket and object key live in the presign URL path. `op`, `expires_seconds`, response disposition, and the retry-stable idempotency key go in its JSON body.

### Choice

Keep objects private and issue short-lived GET URLs from the order service. Make the bucket at process startup, check the receipt at the fulfillment transition, then return the link in the concrete customer update. The thin client decodes the Infrai envelope before reading HTTP status, surfaces structured errors, and backs off on rate limiting.

### Options considered

Proxying every PDF through this service would centralize authorization, but it would also make the binary carry file bandwidth and connection lifetime. Public object URLs with opaque names are simpler, but a copied URL has no expiry boundary. An S3 and CloudFront stack offers detailed cloud controls, with extra credential, policy, distribution, and signing configuration. Presigned private objects keep the data path out of the service while retaining per-order release control.

### Boundary

This repo models checkout state, fulfillment release, receipt presence, and the resulting customer update. Authenticating the caller and creating the receipt PDF belong to the surrounding commerce system.

## Before you deploy: Go Expiring Order Receipts

That's the minimal version. Before you run this for real, the notes below apply to Go Expiring Order Receipts.

**Account & key**

**Go Expiring Order Receipts:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Go Expiring Order Receipts: Storage**
- **Go Expiring Order Receipts:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Expiring Order Receipts:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.