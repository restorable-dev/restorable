import { getPlan, PLAN_LIMITS } from "@/lib/billing/entitlements";
import { createClient } from "@/lib/supabase/server";

import { BillingActions } from "./billing-actions";

export const dynamic = "force-dynamic";

export default async function BillingPage() {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  const plan = user ? await getPlan(supabase, user.id) : "free";
  const limits = PLAN_LIMITS[plan];
  const { data: sub } = await supabase
    .from("subscriptions")
    .select("status, current_period_end, stripe_customer_id")
    .maybeSingle();

  return (
    <main>
      <h1 className="mb-1 text-xl font-semibold">Billing</h1>
      <p className="mb-6 text-sm text-neutral-500">
        The free plan is genuinely usable: one repo, monthly tests, one alert
        channel. Pro removes the limits.
      </p>

      <div className="mb-8 rounded border border-neutral-200 p-4 text-sm dark:border-neutral-800">
        <div className="flex items-center gap-3">
          <span className="text-lg font-semibold">
            {plan === "pro" ? "Pro" : "Free"}
          </span>
          {sub?.status && sub.status !== "inactive" && (
            <span className="rounded bg-neutral-100 px-2 py-0.5 text-xs uppercase text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400">
              {sub.status}
            </span>
          )}
          {plan === "pro" && sub?.current_period_end && (
            <span className="text-xs text-neutral-500">
              renews {new Date(sub.current_period_end).toLocaleDateString()}
            </span>
          )}
        </div>
        <ul className="mt-3 list-inside list-disc text-neutral-600 dark:text-neutral-400">
          <li>
            {limits.maxRepos == null ? "Unlimited repositories" : `${limits.maxRepos} repository`}
          </li>
          <li>
            {limits.minInterval === "any" ? "Any test schedule" : "Monthly restore tests"}
          </li>
          <li>
            {limits.maxAlertChannels == null
              ? "All alert channels"
              : `${limits.maxAlertChannels} alert channel`}
          </li>
          <li>{limits.historyMonths}-month run history</li>
        </ul>
      </div>

      <BillingActions plan={plan} hasCustomer={Boolean(sub?.stripe_customer_id)} />
    </main>
  );
}
