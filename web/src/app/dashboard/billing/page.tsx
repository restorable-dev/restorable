import { getPlan, isBeta, PLAN_LIMITS } from "@/lib/billing/entitlements";
import { createClient } from "@/lib/supabase/server";

import { BillingActions } from "./billing-actions";

export const dynamic = "force-dynamic";

export default async function BillingPage() {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();
  const beta = isBeta();
  const plan = user ? await getPlan(supabase, user.id) : "free";
  // During the beta everyone effectively has Pro limits.
  const limits = beta ? PLAN_LIMITS.pro : PLAN_LIMITS[plan];
  const { data: sub } = await supabase
    .from("subscriptions")
    .select("status, current_period_end, stripe_customer_id")
    .maybeSingle();

  if (beta) {
    return (
      <main>
        <h1 className="mb-1 text-xl font-semibold">Billing</h1>
        <p className="mb-6 text-sm text-neutral-500">
          Restorable is in open beta.
        </p>
        <div className="rounded border border-neutral-200 p-4 text-sm dark:border-neutral-800">
          <div className="flex items-center gap-3">
            <span className="text-lg font-semibold">Beta</span>
            <span className="rounded bg-green-100 px-2 py-0.5 text-xs font-medium uppercase text-green-800 dark:bg-green-900/40 dark:text-green-300">
              all features free
            </span>
          </div>
          <p className="mt-3 text-neutral-600 dark:text-neutral-400">
            Every Pro feature is unlocked while we&apos;re in beta — unlimited
            repositories, any schedule, all alert channels, 12-month history —
            at no cost. No card required. We&apos;ll give plenty of notice
            before paid plans begin.
          </p>
        </div>
      </main>
    );
  }

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
