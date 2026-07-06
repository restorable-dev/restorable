// Signs a synthetic Stripe webhook payload exactly the way Stripe would,
// using the SDK's own test helper, so the handler's signature verification
// runs its real code path.
//
// Usage: node sign-stripe-event.mjs <secret> < payload.json
// Output line 1: the stripe-signature header. The payload passes through
// unchanged (signature covers the exact bytes).
import Stripe from "stripe";

const secret = process.argv[2];
if (!secret) {
  console.error("usage: node sign-stripe-event.mjs <webhook-secret> < payload.json");
  process.exit(1);
}

const chunks = [];
for await (const chunk of process.stdin) {
  chunks.push(chunk);
}
const payload = Buffer.concat(chunks).toString("utf8");

const stripe = new Stripe("sk_test_dummy_key_for_signing_only");
const header = stripe.webhooks.generateTestHeaderString({ payload, secret });
console.log(header);
