// Playwright E2E: User Rankings dashboard tab (spec 003, T009).
//
// Preconditions: enterprise mode. The /api/logs/user-rankings endpoint must
// be reachable. The row-count assertion is conditional — if the test runs
// against an empty log store, the empty-state row renders instead.

import { expect, test } from "@playwright/test";

test.describe("Spec 003 — User Rankings", () => {
	test("tab renders the real view, not a feature-status-panel", async ({ page }) => {
		await page.goto("/workspace/dashboard");
		await page.getByTestId("dashboard-tab-user-rankings").click();

		await expect(page.getByTestId("user-rankings-view")).toBeVisible();
		await expect(page.getByTestId("feature-status-panel")).toHaveCount(0);

		// Either the empty-state row or ≥1 data row renders; both prove
		// the tab is live and not a placeholder.
		const empty = page.getByTestId("user-rankings-empty");
		const rows = page.locator('[data-testid^="user-rankings-row-"]');
		const renderedSomething = (await empty.count()) > 0 || (await rows.count()) > 0;
		expect(renderedSomething).toBe(true);
	});

	test("row click navigates to logs pre-filtered by user_id", async ({ page }) => {
		await page.goto("/workspace/dashboard");
		await page.getByTestId("dashboard-tab-user-rankings").click();
		await expect(page.getByTestId("user-rankings-view")).toBeVisible();

		const firstRow = page.locator('[data-testid^="user-rankings-row-"]').first();
		const rowCount = await firstRow.count();
		if (rowCount === 0) {
			test.skip(true, "no seeded user-attributed requests — drilldown not exercisable");
			return;
		}

		const testid = await firstRow.getAttribute("data-testid");
		const userId = testid?.replace("user-rankings-row-", "") ?? "";
		expect(userId.length).toBeGreaterThan(0);

		await firstRow.click();
		await page.waitForURL(/\/workspace\/logs\?/);
		expect(page.url()).toContain(`user_ids=${encodeURIComponent(userId)}`);
	});
});
