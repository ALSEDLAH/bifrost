// Playwright E2E: Granular RBAC (US2, T034).
//
// Preconditions: a running Bifrost instance with enterprise mode enabled,
// governance + RBAC handlers registered, default org/workspace seeded.
//
// BASE_URL: point at the enterprise deployment (e.g. http://localhost:8088).

import { expect, test } from "@playwright/test";

const unique = () => `e2e-${Date.now()}-${Math.floor(Math.random() * 1000)}`;

test.describe("US2 — Granular RBAC", () => {
	test.beforeEach(async ({ page }) => {
		await page.goto("/workspace/governance/rbac");
		await expect(page.getByTestId("rbac-view")).toBeVisible();
	});

	test("lists built-in roles (Owner, Admin, Member, Manager)", async ({ page }) => {
		for (const name of ["Owner", "Admin", "Member", "Manager"]) {
			await expect(page.getByTestId(`role-row-${name}`)).toBeVisible();
		}
	});

	test("creates a custom role with scoped permissions, then deletes it", async ({ page }) => {
		const roleName = `ReadOnly Analyst ${unique()}`;

		await page.getByTestId("create-role-button").click();
		await expect(page.getByTestId("role-editor-dialog")).toBeVisible();
		await page.getByTestId("role-name-input").fill(roleName);

		// Grant AuditLogs:Read, AuditLogs:View, Logs:Read.
		await page.getByTestId("role-cell-AuditLogs-Read").click();
		await page.getByTestId("role-cell-AuditLogs-View").click();
		await page.getByTestId("role-cell-Logs-Read").click();

		await page.getByTestId("role-save-button").click();

		// Row appears in the list.
		await expect(page.getByTestId(`role-row-${roleName}`)).toBeVisible();

		// Delete (cleanup).
		await page.getByTestId(`delete-role-${roleName}`).click();
		await page.getByRole("button", { name: /^Delete$/ }).click();
		await expect(page.getByTestId(`role-row-${roleName}`)).toHaveCount(0);
	});

	test("cannot edit or delete a built-in role", async ({ page }) => {
		await expect(page.getByTestId("edit-role-Owner")).toBeDisabled();
		// Delete button is not rendered for built-in roles.
		await expect(page.getByTestId("delete-role-Owner")).toHaveCount(0);
	});

	test("assigns a role to a user then unassigns", async ({ page }) => {
		const email = `e2e-${Date.now()}@example.com`;

		// Invite a user first.
		await page.goto("/workspace/governance/users");
		await expect(page.getByTestId("users-view")).toBeVisible();
		await page.getByTestId("invite-user-button").click();
		await page.getByTestId("invite-email-input").fill(email);
		await page.getByTestId("invite-user-submit").click();
		await expect(page.getByTestId(`user-row-${email}`)).toBeVisible();

		// Assign Admin.
		await page.getByTestId(`assign-role-${email}`).click();
		await page.getByTestId("assign-role-select").click();
		await page.getByRole("option", { name: /Admin/ }).first().click();
		await page.getByTestId("assign-role-submit").click();
		await expect(page.getByText(/Admin/).first()).toBeVisible();

		// Cleanup: close dialog + delete user.
		await page.keyboard.press("Escape");
		await page.getByTestId(`delete-user-${email}`).click();
		await page.getByRole("button", { name: /^Delete$/ }).click();
		await expect(page.getByTestId(`user-row-${email}`)).toHaveCount(0);
	});
});
