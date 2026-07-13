import type { SupabaseClient } from "@supabase/supabase-js";

// Plan entitlements (SPEC pricing). Enforced SERVER-SIDE at the point of
// action — repo creation, channel creation — never in client code. The free
// tier must stay genuinely usable: limits cap growth, they never break or
// hide existing data.

export type Plan = "free" | "pro";

export interface PlanLimits {
  // null = unlimited
  maxRepos: number | null;
  maxAlertChannels: number | null;
  minInterval: "monthly" | "any";
  historyMonths: number;
}

export const PLAN_LIMITS: Record<Plan, PlanLimits> = {
  free: { maxRepos: 1, maxAlertChannels: 1, minInterval: "monthly", historyMonths: 3 },
  pro: { maxRepos: null, maxAlertChannels: null, minInterval: "any", historyMonths: 12 },
};

// Stripe statuses that keep Pro entitlements. past_due keeps them during
// Stripe's retry window — cutting features over a failed card retry is
// hostile; deletion (canceled) is what downgrades.
const ACTIVE_STATUSES = new Set(["active", "trialing", "past_due"]);

export interface SubscriptionRow {
  plan: Plan;
  status: string;
  stripe_customer_id: string;
  stripe_subscription_id: string | null;
  current_period_end: string | null;
}

// getPlan works with either the user-scoped client (RLS) or the admin
// client; subscriptions are readable by their owner.
export async function getPlan(client: SupabaseClient, userId: string): Promise<Plan> {
  const { data } = await client
    .from("subscriptions")
    .select("plan, status")
    .eq("user_id", userId)
    .maybeSingle();
  if (data?.plan === "pro" && ACTIVE_STATUSES.has(data.status)) {
    return "pro";
  }
  return "free";
}

export async function getLimits(client: SupabaseClient, userId: string): Promise<PlanLimits> {
  // Free-for-all beta: everyone gets Pro limits, no payment. Flip BETA_MODE
  // off (and re-apply the DB channel-limit policy) when paid plans go live.
  if (isBeta()) {
    return PLAN_LIMITS.pro;
  }
  return PLAN_LIMITS[await getPlan(client, userId)];
}

// isBeta reports whether the hosted service is running its free-for-all beta.
export function isBeta(): boolean {
  return process.env.BETA_MODE === "1";
}
