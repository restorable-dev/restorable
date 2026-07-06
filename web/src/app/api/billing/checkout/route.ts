import { z } from "zod";

import { getStripe, priceIdFor } from "@/lib/billing/stripe";
import { createAdminClient } from "@/lib/supabase/admin";
import { createClient } from "@/lib/supabase/server";

const bodySchema = z.object({ interval: z.enum(["month", "year"]) });

// POST /api/billing/checkout — creates a Stripe Checkout session for the
// signed-in user and returns its URL.
export async function POST(request: Request) {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  if (!user) {
    return Response.json({ error: "not signed in" }, { status: 401 });
  }

  const parsed = bodySchema.safeParse(await request.json().catch(() => null));
  if (!parsed.success) {
    return Response.json({ error: "interval must be month or year" }, { status: 400 });
  }

  const stripe = getStripe();
  const admin = createAdminClient();

  // Reuse the Stripe customer across sessions; create on first checkout.
  const { data: existing } = await admin
    .from("subscriptions")
    .select("stripe_customer_id")
    .eq("user_id", user.id)
    .maybeSingle();
  let customerId = existing?.stripe_customer_id;
  if (!customerId) {
    const customer = await stripe.customers.create({
      email: user.email ?? undefined,
      metadata: { user_id: user.id },
    });
    customerId = customer.id;
    const { error } = await admin.from("subscriptions").upsert(
      { user_id: user.id, stripe_customer_id: customerId },
      { onConflict: "user_id" },
    );
    if (error) {
      return Response.json({ error: "could not prepare checkout" }, { status: 500 });
    }
  }

  const appUrl = process.env.NEXT_PUBLIC_APP_URL ?? new URL(request.url).origin;
  const session = await stripe.checkout.sessions.create({
    mode: "subscription",
    customer: customerId,
    line_items: [{ price: priceIdFor(parsed.data.interval), quantity: 1 }],
    client_reference_id: user.id,
    success_url: `${appUrl}/dashboard/billing?success=1`,
    cancel_url: `${appUrl}/dashboard/billing?canceled=1`,
  });
  return Response.json({ url: session.url });
}
