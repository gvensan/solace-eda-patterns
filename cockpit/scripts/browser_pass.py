#!/usr/bin/env python3
"""Drive scenario pages in a headless browser, the way an attendee would.

For each scenario: open its page, press Play, then each Break it card's
Break it and Reset, then Cleanup, saving a screenshot after each step. It
fails if the page logs a console error or an uncaught exception, if a
request fails, or if the dashboard's API answers with an error. smoke.py
checks what the apps print; this checks that the page itself works.

    ${COCKPIT_VENV:-cockpit/.venv}/bin/python cockpit/scripts/browser_pass.py <scenario-id> [...]

Needs Playwright (cockpit/requirements-dev.txt). It uses the installed
Google Chrome when there is one, otherwise Playwright's own Chromium
(`playwright install chromium`). Screenshots go to
cockpit/.state/browser-pass/<time>/, which git ignores; the path is
printed at the end. COCKPIT_URL overrides http://localhost:3000, and
BREAK_SECONDS (default 12) is how long to watch each card before Reset.

Close the dashboard's own tabs first: like smoke.py, this presses buttons.
"""

import os
import pathlib
import sys
import time

from playwright.sync_api import sync_playwright

BASE = os.environ.get("COCKPIT_URL", "http://localhost:3000")
BREAK_SECONDS = float(os.environ.get("BREAK_SECONDS", "12"))
OUT = pathlib.Path(__file__).resolve().parent.parent / ".state" / "browser-pass" / time.strftime("%Y%m%d-%H%M%S")


def sequence_done(page, sid: str, timeout: float = 300) -> None:
    """Wait until the dashboard has finished Play or Cleanup for sid."""
    deadline = time.time() + timeout
    time.sleep(1)
    while time.time() < deadline:
        runs = page.request.get(f"{BASE}/api/runs").json()
        if sid not in runs["sequences"]:
            return
        time.sleep(1)
    raise TimeoutError(f"{sid}: still in Play or Cleanup after {timeout:g}s")


def badge_of(card) -> str:
    return (card.locator(".fm__badge").text_content() or "").strip()


def run(page, sid: str, problems: list) -> None:
    folder = OUT / sid
    folder.mkdir(parents=True, exist_ok=True)
    def shot(name: str) -> None:
        """The page as it stands, and the diagram on its own beside it."""
        page.screenshot(path=str(folder / f"{name}.png"), full_page=True)
        figure = page.locator(".dg-figure")
        if figure.count():
            figure.first.screenshot(path=str(folder / f"{name}-diagram.png"))

    page.goto(f"{BASE}/#{sid}")
    page.wait_for_selector(".dg-figure", timeout=15000)
    shot("00-open")

    page.click("#tp-play")
    sequence_done(page, sid)
    err = page.request.get(f"{BASE}/api/runs").json()["sequenceErrors"].get(sid)
    if err:
        problems.append(f"{sid}: Play stopped: {err}")
        return
    time.sleep(6)
    shot("01-play")

    cards = page.locator(".fm")
    for i in range(cards.count()):
        card = cards.nth(i)
        title = (card.locator(".fm__title").first.text_content() or f"card {i + 1}").strip()
        card.scroll_into_view_if_needed()
        card.get_by_role("button", name="Break it", exact=True).click()
        time.sleep(BREAK_SECONDS)
        if badge_of(card) != "Broken":
            problems.append(f"{sid}: {title}: badge says {badge_of(card)!r} after Break it, expected Broken")
        shot(f"{2 + 2 * i:02d}-card{i + 1}-broken")
        card.get_by_role("button", name="Reset", exact=True).click()
        time.sleep(BREAK_SECONDS)
        if badge_of(card) not in ("Recovered", "Ready"):
            problems.append(f"{sid}: {title}: badge says {badge_of(card)!r} after Reset")
        shot(f"{3 + 2 * i:02d}-card{i + 1}-reset")

    page.click("#tp-cleanup")
    sequence_done(page, sid)
    time.sleep(2)
    shot("99-cleanup")


def main(ids) -> int:
    if not ids:
        print("usage: browser_pass.py <scenario-id> [...]")
        return 2
    problems: list = []
    with sync_playwright() as p:
        try:
            browser = p.chromium.launch(channel="chrome", headless=True)
        except Exception:
            browser = p.chromium.launch(headless=True)
        context = browser.new_context(viewport={"width": 1440, "height": 1000},
                                      extra_http_headers={"X-Workshop-Client": "browser-pass"})
        page = context.new_page()
        current = {"sid": "?"}
        page.on("console", lambda m: m.type == "error"
                and problems.append(f"{current['sid']}: console error: {m.text}"))
        page.on("pageerror", lambda e: problems.append(f"{current['sid']}: page error: {e}"))
        page.on("requestfailed", lambda r: problems.append(
            f"{current['sid']}: request failed: {r.method} {r.url} ({r.failure})"))
        page.on("response", lambda r: r.status >= 400 and "/api/" in r.url
                and problems.append(f"{current['sid']}: {r.status} from {r.request.method} {r.url}"))
        # Cleanup and Clear ask first; an attendee says yes.
        page.on("dialog", lambda d: d.accept())
        for sid in ids:
            current["sid"] = sid
            print(f"=== {sid}", flush=True)
            try:
                run(page, sid, problems)
            except Exception as exc:
                problems.append(f"{sid}: {type(exc).__name__}: {exc}")
        browser.close()
    for msg in problems:
        print(f"  FAIL {msg}")
    print(f"\nBrowser pass over {len(ids)} scenario(s), {len(problems)} problem(s). Screenshots: {OUT}")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
