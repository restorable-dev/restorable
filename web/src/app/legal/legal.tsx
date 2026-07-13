import Link from "next/link";

// Shared shell for the Terms and Privacy pages.
export function LegalPage({
  title,
  updated,
  children,
}: {
  title: string;
  updated: string;
  children: React.ReactNode;
}) {
  return (
    <main className="mx-auto w-full max-w-2xl px-6 py-16">
      <Link href="/" className="text-sm text-neutral-500 underline">
        ← Restorable
      </Link>
      <h1 className="mt-6 text-2xl font-semibold">{title}</h1>
      <p className="mt-1 text-sm text-neutral-500">Last updated: {updated}</p>
      <div className="prose mt-8 flex flex-col gap-4 text-sm leading-relaxed text-neutral-700 dark:text-neutral-300 [&_h2]:mt-6 [&_h2]:font-semibold [&_h2]:text-neutral-900 dark:[&_h2]:text-neutral-100 [&_ul]:list-disc [&_ul]:pl-5">
        {children}
      </div>
    </main>
  );
}
