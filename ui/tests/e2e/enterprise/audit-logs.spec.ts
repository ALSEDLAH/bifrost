// Playwright E2E: Audit Logs (US4, T037).
//
// Preconditions: enterprise mode with governance + RBAC + audit plugins
// initialized. Admin actions (role/user CRUD) emit audit entries.

import { expect, test } from "@playwright/test";

test.describe("US4 — Audit Logs", () => {
	test("view renders with filter row and export buttons", async ({ page }) => {
		await page.goto("/workspace/audit-logs");
		await expect(page.getByTestId("audit-logs-view")).toBeVisible();
		await expect(page.getByTestId("audit-filter-actor")).toBeVisible();
		await expect(page.getByTestId("audit-filter-action")).toBeVisible();
		await expect(page.getByTestId("audit-filter-resource")).toBeVisible();
		await expect(page.getByTestId("audit-filter-outcome")).toBeVisible();
		await expect(page.getByTestId("audit-filter-from")).toBeVisible();
		await expect(page.getByTestId("audit-filter-to")).toBeVisible();
		await expect(page.getByTestId("audit-export-csv")).toBeVisible();
		await expect(page.getByTestId("audit-export-json")).toBeVisible();
	});

	test("admin actions produce audit entries visible in the log", async ({ page }) => {
		// Perform a trackable action: create + delete a role.
		const roleName = `audit-probe-${Date.now()}`;
		await page.goto("/workspace/governance/rbac");
		await page.getByTestId("create-role-button").click();
		await page.getByTestId("role-name-input").fill(roleName);
		await page.getByTestId("role-cell-AuditLogs-Read").click();
		await page.getByTestId("role-save-button").click();
		await expect(page.getByTestId(`role-row-${roleName}`)).toBeVisible();
		await page.getByTestId(`delete-role-${roleName}`).click();
		await page.getByRole("button", { name: /^Delete$/ }).click();

		// Navigate to audit logs and filter.
		await page.goto("/workspace/audit-logs");
		await expect(page.getByTestId("audit-logs-view")).toBeVisible();
		await page.getByTestId("audit-filter-resource").fill("role");
		await page.getByTestId("audit-apply-filters").click();

		// Expect at least one row referencing the role resource.
		await expect(page.getByRole("cell", { name: /role/i }).first()).toBeVisible();
	});

	test("filter reset clears all input values", async ({ page }) => {
		await page.goto("/workspace/audit-logs");
		await page.getByTestId("audit-filter-actor").fill("some-user-id");
		await page.getByTestId("audit-filter-action").fill("role.create");
		await page.getByTestId("audit-reset-filters").click();
		await expect(page.getByTestId("audit-filter-actor")).toHaveValue("");
		await expect(page.getByTestId("audit-filter-action")).toHaveValue("");
	});

	test("CSV export opens the export endpoint", async ({ page, context }) => {
		await page.goto("/workspace/audit-logs");
		const [popup] = await Promise.all([
			context.waitForEvent("page"),
			page.getByTestId("audit-export-csv").click(),
		]);
		await popup.waitForLoadState("domcontentloaded");
		expect(popup.url()).toMatch(/\/api\/audit-logs\/export\?.*format=csv/);
		await popup.close();
	});
});
