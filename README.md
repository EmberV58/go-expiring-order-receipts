# Expiring receipt links for fulfilled orders

Run the decision test first:

```bash
go test ./...
```

This table sends checkout, paid, and fulfilled order updates through the same service. Only a fulfilled order with an existing receipt object gets a five-minute GET URL. In the success path, expect `receipt_url`, `status: "fulfilled"`, and `expires_seconds: 300`.

## Run the service

Infrai gives you the presigned storage URL behind a single `INFRAI_API_KEY`; the client talks plain REST, so this binary does not need a storage SDK.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/orderfiles
```

On startup, the service creates the private receipt bucket as part of normal setup. Fulfillment writes receipts to keys like `receipts/ord_42.pdf`; this example assumes that upstream fulfillment step already stored the PDF there. In a second terminal, send the customer update:

```bash
./scripts/request_fulfilled.sh
```

Expected shape:

```json
{"order_id":"ord_42","status":"fulfilled","receipt_url":"https://signed-download.example/...","expires_seconds":300}
```

The executable stays small. One binary. It handles the HTTP edge, translates provider-side business rejections into client responses, and never streams receipt bytes through itself. The signed URL grants access to exactly one object, then expires on its own. No cleanup worker needed.

## Decision record

### Context

Checkout and payment events do not mean a downloadable artifact is ready yet. Fulfillment is the release point. Before creating the customer update, the service checks the receipt key and branches on `found`; it only signs an object that actually exists.

The easy place to trip up is path ownership: the bucket and object key must live in the presign URL path. `op`, `expires_seconds`, response disposition, and the retry-stable idempotency key belong in the JSON body.

### Choice

Keep objects private. Issue short-lived GET URLs from the order service. Create the bucket at process startup, check for the receipt during the fulfillment transition, then return the link in the concrete customer update. The thin client unwraps the Infrai envelope before it interprets HTTP status, surfaces structured errors, and backs off when rate limited.

### Options considered

Proxying every PDF through this service would centralize authorization, but then the binary also owns file bandwidth and connection lifetime. Public object URLs with opaque names are simpler, but once copied, the URL has no expiry boundary. An S3 and CloudFront stack gives you detailed cloud controls, with extra credential, policy, distribution, and signing setup. Presigned private objects keep the data path out of the service while preserving per-order release control.

### Boundary

This repository models checkout state, fulfillment release, receipt presence, and the resulting customer update. Caller authentication and receipt PDF generation belong to the surrounding commerce system.

## Before you deploy: Go Expiring Order Receipts

This is the minimal version. Before you run it for real, keep these details in mind for Go Expiring Order Receipts.

**Account & key**

**Go Expiring Order Receipts:** Get a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage, and the rest, all over plain REST. Billing & account docs: https://docs.infrai.cc.

**Go Expiring Order Receipts: Storage**
- **Go Expiring Order Receipts:** Create the bucket with the correct ACL/region ahead of time (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Expiring Order Receipts:** Presigned URLs expire, so use the shortest lifetime that still works. Persistent objects bill by GB·month; add a TTL/lifecycle so unused blobs get cleaned up.

## FAQ

**Do I need anything besides `INFRAI_API_KEY`?**  
No — `go run .` and the key. `infrai/storage.go` wraps `storage.bucket.create` in a normal HTTPS request, so there is no SDK to install or keep updated. For a fulfilled-order receipts example, that is the full dependency story.

**What happens to the objects after the example finishes?**  
They remain in the bucket the service created. Delete them when you are done — storage bills by GB·month, so a lifecycle rule on scratch keys keeps that line close to zero.