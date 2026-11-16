"""Screenshots the retired v0.1 dashboard, seeded with synthetic data.

    python3 showcase/shoot.py <base_url> <email> <password> <fail_run_id> <out_dir>
"""

import sys
from pathlib import Path

from playwright.sync_api import sync_playwright

base, email, password, fail_run, out_dir = sys.argv[1:6]
out = Path(out_dir)
out.mkdir(parents=True, exist_ok=True)

pages = [
    ("dashboard-overview", "/dashboard"),
    ("dashboard-runs", "/dashboard/runs"),
    ("dashboard-run-failed", f"/dashboard/runs/{fail_run}"),
    ("dashboard-agents", "/dashboard/agents"),
    ("dashboard-alerts", "/dashboard/alerts"),
]

with sync_playwright() as p:
    browser = p.chromium.launch()
    for scale_name, viewport in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
        ctx = browser.new_context(viewport=viewport, device_scale_factor=2)
        page = ctx.new_page()

        page.goto(base + "/", wait_until="networkidle")
        page.screenshot(path=str(out / f"landing-{scale_name}.png"), full_page=True)

        page.goto(base + "/login", wait_until="networkidle")
        page.fill('input[type="email"]', email)
        page.fill('input[type="password"]', password)
        page.click('button[type="submit"]')
        page.wait_for_url("**/dashboard**", timeout=30000)

        for name, path in pages:
            page.goto(base + path, wait_until="networkidle")
            page.wait_for_timeout(800)
            # Clip to the content, not the viewport: these pages are short.
            height = page.evaluate("Math.ceil(document.querySelector('main, body').getBoundingClientRect().bottom) + 48")
            page.set_viewport_size({"width": viewport["width"], "height": max(480, min(height, 2400))})
            page.screenshot(path=str(out / f"{name}-{scale_name}.png"), full_page=False)
            page.set_viewport_size(viewport)
            print("captured", name, scale_name)
        ctx.close()
    browser.close()
