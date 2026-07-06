import "server-only";

import Stripe from "stripe";

// Server-side Stripe client. The key is test-mode in development; the same
// code serves live mode purely through environment configuration.
export function getStripe(): Stripe {
  const key = process.env.STRIPE_SECRET_KEY;
  if (!key) {
    throw new Error("STRIPE_SECRET_KEY is not set");
  }
  return new Stripe(key);
}

export function priceIdFor(interval: "month" | "year"): string {
  const id =
    interval === "month"
      ? process.env.STRIPE_PRICE_MONTHLY
      : process.env.STRIPE_PRICE_YEARLY;
  if (!id) {
    throw new Error(`price id for ${interval} is not configured`);
  }
  return id;
}
