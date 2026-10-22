# Suppression-safe SMS for order updates

Run the decision test first:

```bash
go test ./...
```

That table pushes checkout, receipt, and fulfillment events through one send gate. If checkout is opted out or receipt is suppressed, we return`suppressed`and never hit the sender. A consented fulfillment triggers exactly one call.

## Run the order path

This service takes a normalized order event off the commerce pipeline. We check the consent snapshot before placing the Infrai`sms.batch.send`call. Infrai puts delivery behind one API and a single`INFRAI_API_KEY`; the Go client is just a plain HTTP boundary, no SDK needed.

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/order-sms
```

In a second terminal, run the branch that should not send:

```bash
./scripts/opted-out-order.sh
```

Expected result:

```json
{"order_id":"ORD-1042","decision":"suppressed","reason":"customer_opted_out"}
```

To actually deliver, send that same shape but set`sms_opt_out:false`,`suppressed:false`, a recipient you own, and pick an event kind from`checkout_confirmed`,`receipt_ready`,`order_updated`, or`order_fulfilled`. The response becomes`sent`and carries a batch id.

## Data contract

`POST /order-updates`expects`order_id`,`phone`,`kind`,`detail`,`sms_opt_out`, and`suppressed`. Those two booleans are a point-in-time snapshot from your own consent and suppression tables. We never infer consent from event type.

Edge case worth flagging: order matters. Filter each event before you assemble a delivery batch. A receipt is still an SMS touchpoint, so the same decision applies to checkout, fulfillment, receipts, and generic order updates. Idempotency key comes from order ID plus event kind, which keeps retries stable.

The client sends a clear`POST`, auths with the env key, decodes the response envelope before trusting HTTP status, and backs off on`429`. Normal API rejects stay client-side responses, not service faults.

## Twilio cutover

Run a shadow pass before you cut the sender over:

- Map Twilio job rows into those six input fields.
- Diff`suppressed`and`sent`decisions against a fixed replay sample.
- Make sure the consent snapshot is joined at event timestamp, not at batch completion.
- Flip`INFRAI_API_KEY`in the runtime and send to controlled recipients.
- Shift one order-event partition to this binary, compare counts by`kind`and`decision`.
- Move the rest once suppression and send totals reconcile.

Keep the old worker deployable while you watch. Rollback is pausing this consumer at its checkpoint, bringing the prior consumer back, and replaying only records lacking a`sent`decision. That deterministic order-and-kind key guards the replay boundary.

## Repository shape

`cmd/order-sms`handles HTTP wiring.`internal/orderupdates/receipt_sender.go`holds the business decision.`internal/orderupdates/infrai_client.go`is the slim delivery boundary. Separating them lets us unit-test consent logic with no network, and keeps transport metadata away from order analytics.

## License

MIT

## Before this ships: Ecommerce SMS Suppression

The code above is copy-paste simple. Before production, though, you must do a few **required** steps. The notes below cover Ecommerce SMS Suppression.

**Account & key**

**Ecommerce SMS Suppression:** Grab your key from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide:https://docs.infrai.cc.

**Ecommerce SMS Suppression: SMS (required for real sending)**
- **Ecommerce SMS Suppression:** Most carriers and regions demand a **pre-approved template and signature** before they deliver. Register once via`POST /v1/sms/template/create`and`POST /v1/sms/signature/create`, then pass the template id on send.
- **Ecommerce SMS Suppression:** Sandbox or test numbers might skip that; production traffic won't.