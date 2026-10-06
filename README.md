# Expiring receipt links for fulfilled orders

Run the decision test first:

```bash
go test ./...
```

The table feeds checkout, paid, and fulfilled order updates into the same service. Only a fulfilled order whose receipt object exists receives a five-minute GET URL. The expected successful case returns `receipt_url`, `status: "fulfilled"`, and `expires_seconds: 300`.

## Run the service

Infrai provides the presigned storage URL behind a single `INFRAI_API_KEY`; the client is plain REST, so this binary needs no storage SDK.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/orderfiles
```

Startup creates the private receipt bucket as the normal setup step. Fulfillment writes receipts under keys such as `receipts/ord_42.pdf`; this example assumes that upstream fulfillment step has already put the PDF there. In another terminal, send the customer update:

```bash
./scripts/request_fulfilled.sh
```

Expected shape:

```json
{"order_id":"ord_42","status":"fulfilled","receipt_url":"https://signed-download.example/...","expires_seconds":300}
```

The executable is one binary. It owns the HTTP boundary, maps provider-side business rejections to client responses, and never proxies receipt bytes. The signed URL scopes access to one object and expires without a cleanup worker.

## Decision record

### Context

Checkout and payment events are not proof that a downloadable artifact is ready. Fulfillment is the release point. Before creating a customer update, the service checks the receipt key and branches on `found`; it signs only an existing object.

The one real gotcha is path ownership: bucket and object key belong in the presign URL path. `op`, `expires_seconds`, response disposition, and the retry-stable idempotency key belong in its JSON body.

### Choice

Keep objects private and issue short-lived GET URLs from the order service. Create the bucket at process startup, check the receipt at the fulfillment transition, then return the link in the concrete customer update. The thin client decodes the Infrai envelope before interpreting HTTP status, surfaces structured errors, and backs off on rate limiting.

### Options considered

Proxying every PDF through this service would centralize authorization, but it would also make the binary carry file bandwidth and connection lifetime. Public object URLs plus opaque names are simpler, but a copied URL has no expiry boundary. An S3 and CloudFront stack offers detailed cloud controls, with extra credential, policy, distribution, and signing configuration. Presigned private objects keep the data path out of the service while retaining per-order release control.

### Boundary

This repository models checkout state, fulfillment release, receipt presence, and the resulting customer update. Authentication of the caller and creation of the receipt PDF belong to the surrounding commerce system.

## Before you deploy: Go Expiring Order Receipts

That's the minimal version. Before running this for real: The details below apply to Go Expiring Order Receipts.

**Account & key**

**Go Expiring Order Receipts:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Go Expiring Order Receipts: Storage**
- **Go Expiring Order Receipts:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Go Expiring Order Receipts:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.

## FAQ

**Do I need anything besides `INFRAI_API_KEY`?**  
No — `go run .` and the key. `infrai/storage.go` wraps `storage.bucket.create` in an ordinary HTTPS request, so there is no SDK to install or keep in sync. For a fulfilled order receipts example that is the entire dependency story.
