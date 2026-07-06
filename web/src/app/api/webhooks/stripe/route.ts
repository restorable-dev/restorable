import { z } from "zod";

import { getStripe } from "@/lib/billing/stripe";
import { createAdminClient } from "@/lib/supabase/admin";

// POST /api/webhooks/stripe
//
// Contract (SPEC): zod-validated and idempotent. Signature verification
// proves the payload came from Stripe; zod re-validates the exact fields we
// consume; the stripe_events primary key makes every event process exactly
// once — a replayed delivery insert-conflicts and returns early.

const checkoutSessionSchema = z.object({
  id: z.string(),
  customer: z.string(),
  subscription: z.string().nullish(),
  client_reference_id: z.string().nullish(),
});

const subscriptionSchema = z.object({
  id: z.string(),
  customer: z.string(),
  status: z.string(),
  current_period_end: z.number().nullish(),
  items: z
    .object({
      data: z.array(z.object({ current_period_end: z.number().nullish() }).loose()),
    })
    .loose()
    .nullish(),
});

function periodEnd(sub: z.infer<typeof subscriptionSchema>): string | null {
  const epoch = sub.current_period_end ?? sub.items?.data?.[0]?.current_period_end;
  return epoch ? new Date(epoch * 1000).toISOString() : null;
}

export async function POST(request: Request) {
  const secret = process.env.STRIPE_WEBHOOK_SECRET;
  if (!secret) {
    return Response.json({ error: "webhook not configured" }, { status: 500 });
  }
  const signature = request.headers.get("stripe-signature");
  if (!signature) {
    return Response.json({ error: "missing signature" }, { status: 400 });
  }

  const body = await request.text();
  let event;
  try {
    event = await getStripe().webhooks.constructEventAsync(body, signature, secret);
  } catch {
    return Response.json({ error: "invalid signature" }, { status: 400 });
  }

  const admin = createAdminClient();

  // Idempotency: first delivery wins, replays are acknowledged no-ops.
  const { error: ledgerError } = await admin
    .from("stripe_events")
    .insert({ id: event.id, type: event.type });
  if (ledgerError) {
    if (ledgerError.code === "23505") {
      return Response.json({ received: true, replay: true });
    }
    return Response.json({ error: "event ledger failure" }, { status: 500 });
  }

  switch (event.type) {
    case "checkout.session.completed": {
      const parsed = checkoutSessionSchema.safeParse(event.data.object);
      if (!parsed.success) {
        return Response.json({ error: "unexpected checkout payload" }, { status: 400 });
      }
      const session = parsed.data;
      if (!session.client_reference_id) {
        break; // not one of ours
      }
      const { error } = await admin.from("subscriptions").upsert(
        {
          user_id: session.client_reference_id,
          stripe_customer_id: session.customer,
          stripe_subscription_id: session.subscription ?? null,
          plan: "pro",
          status: "active",
          updated_at: new Date().toISOString(),
        },
        { onConflict: "user_id" },
      );
      if (error) {
        return Response.json({ error: "subscription update failed" }, { status: 500 });
      }
      break;
    }

    case "customer.subscription.updated": {
      const parsed = subscriptionSchema.safeParse(event.data.object);
      if (!parsed.success) {
        return Response.json({ error: "unexpected subscription payload" }, { status: 400 });
      }
      const sub = parsed.data;
      const { error } = await admin
        .from("subscriptions")
        .update({
          stripe_subscription_id: sub.id,
          status: sub.status,
          current_period_end: periodEnd(sub),
          updated_at: new Date().toISOString(),
        })
        .eq("stripe_customer_id", sub.customer);
      if (error) {
        return Response.json({ error: "subscription update failed" }, { status: 500 });
      }
      break;
    }

    case "customer.subscription.deleted": {
      const parsed = subscriptionSchema.safeParse(event.data.object);
      if (!parsed.success) {
        return Response.json({ error: "unexpected subscription payload" }, { status: 400 });
      }
      const sub = parsed.data;
      // Graceful downgrade: plan flips to free, data stays untouched.
      const { error } = await admin
        .from("subscriptions")
        .update({
          plan: "free",
          status: "canceled",
          current_period_end: periodEnd(sub),
          updated_at: new Date().toISOString(),
        })
        .eq("stripe_customer_id", sub.customer);
      if (error) {
        return Response.json({ error: "subscription update failed" }, { status: 500 });
      }
      break;
    }

    default:
      // Acknowledged but not consumed; the ledger still records it.
      break;
  }

  return Response.json({ received: true });
}
