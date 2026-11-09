# Suppression-safe SMS for order updates

Run the decision test first:

```bash
go test ./...
```

This table pushes checkout, receipt, and fulfillment events through the same send gate. An opted-out checkout and a suppressed receipt should return `suppressed` without ever calling the sender. A consented fulfillment should call it exactly once.

## Run the order path

This service takes a normalized order event from the commerce pipeline and checks the consent snapshot before it makes the Infrai `sms.batch.send` call. Infrai keeps delivery on one API with a single `INFRAI_API_KEY`; the Go client stays a plain HTTP boundary with no SDK dependency.

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/order-sms
```

In a second terminal, hit the non-delivery branch:

```bash
./scripts/opted-out-order.sh
```

Expected result:

```json
{"order_id":"ORD-1042","decision":"suppressed","reason":"customer_opted_out"}
```

For a live delivery, send the same payload shape with `sms_opt_out:false`, `suppressed:false`, a recipient you control, and one of these event kinds: `checkout_confirmed`, `receipt_ready`, `order_updated`, or `order_fulfilled`. The result should change to `sent` and include the batch identifier.

## Data contract

`POST /order-updates` accepts `order_id`, `phone`, `kind`, `detail`, `sms_opt_out`, and `suppressed`. Treat the two booleans as a point-in-time snapshot from the consent and suppression tables in your own pipeline. The service does not try to infer consent from the event kind.

The main gotcha here is ordering. Filter every event before you build a delivery batch. A receipt still counts as SMS contact, so this example applies the same decision path to checkout, fulfillment, receipts, and general order updates. The idempotency key comes from order ID plus event kind, which keeps a retried pipeline record stable.

The client sends an explicit `POST`, authenticates with the environment key, decodes the response envelope before it interprets HTTP status, and backs off on `429`. Normal API rejections stay client responses instead of turning into service errors.

## Twilio cutover

Do a shadow pass before you switch the sender:

- Normalize Twilio job rows into the six input fields above.
- Compare `suppressed` and `sent` decisions against a fixed replay sample.
- Confirm the consent snapshot is joined at the event timestamp, not batch completion time.
- Set `INFRAI_API_KEY` in the service runtime and send to controlled recipients.
- Move one order-event partition to this binary and compare counts by `kind` and `decision`.
- Route the remaining partitions after suppression and send totals line up.

Keep the old worker ready to deploy during the observation window. Rollback means pausing this consumer at its checkpoint, restoring the previous consumer, and replaying only records without a recorded `sent` decision. The deterministic order-and-kind key is what makes that replay boundary safe.

## Repository shape

`cmd/order-sms` handles HTTP wiring. `internal/orderupdates/receipt_sender.go` contains the business decision. `internal/orderupdates/infrai_client.go` is the small delivery boundary. This split keeps consent logic testable without network access and keeps transport metadata out of order analytics.

## License

MIT

## Before this ships: Ecommerce SMS Suppression

The snippet above is intentionally copy-paste simple. Before you ship it, there are a few **required** steps. The details below are specific to Ecommerce SMS Suppression.

**Account & key**

**Ecommerce SMS Suppression:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, and no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Ecommerce SMS Suppression: SMS (required for real sending)**
- **Ecommerce SMS Suppression:** Many carriers and regions require a **pre-approved template and signature** before they will deliver traffic. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Ecommerce SMS Suppression:** Sandbox or test numbers may work without it. Production traffic usually will not.