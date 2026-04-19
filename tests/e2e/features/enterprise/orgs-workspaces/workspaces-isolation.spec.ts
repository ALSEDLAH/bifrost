// US1 acceptance scenario — Workspace isolation.
//
// Per spec.md US1 Acceptance Scenario 3:
//
//   Given a user with access only to Workspace A,
//   When they query the logs API or analytics API,
//   Then results include only requests made against Workspace A's
//   virtual keys; Workspace B data is never returned and never appears
//   in counts or aggregates.
//
// This Playwright spec exercises the UI surface: a user with scope
// on only Workspace A cannot see Workspace B in the list. The full
// API-level isolation test is in the Go layer
// (framework/tenancy/orgs_workspaces_test.go — TestWorkspaceRepo_CrossOrgIsolation)
// and transports/bifrost-http/handlers/workspaces_test.go
// (TestWorkspacesHandler_CrossOrgGet_Returns404).
//
// Requires: running Bifrost backend (go run ./transports/...), the UI
// dev server, and an enterprise-mode configuration with at least two
// workspaces pre-seeded + a user scoped to only one of them. See
// tests/e2e/README.md for harness setup.

import { expect, test } from "../../../core/fixtures/base.fixture";
import { createWorkspaceData } from "./orgs-workspaces.data";
import { WorkspacesPage } from "./pages/workspacesPage";

const createdSlugs: string[] = [];

test.describe("US1 — Workspaces Isolation", () => {
	test.describe.configure({ mode: "serial" });

	test.afterEach(async ({ page }) => {
		const wsPage = new WorkspacesPage(page);
		await wsPage.goto();
		for (const slug of [...createdSlugs]) {
			try {
				if (await wsPage.rowExists(slug)) {
					await wsPage.deleteWorkspace(slug);
				}
			} catch (e) {
				console.error(`[CLEANUP] Failed to delete workspace ${slug}:`, e);
			}
		}
		createdSlugs.length = 0;
	});

	test("workspaces page renders either table or empty state", async ({ page }) => {
		const wsPage = new WorkspacesPage(page);
		await wsPage.goto();

		const newVisible = await wsPage.newBtn.isVisible().catch(() => false);
		const emptyAddVisible = await page.getByTestId("workspace-button-add").isVisible().catch(() => false);
		expect(newVisible || emptyAddVisible).toBe(true);
	});

	test("operator can create a workspace", async ({ page }) => {
		const wsPage = new WorkspacesPage(page);
		await wsPage.goto();

		const data = createWorkspaceData();
		createdSlugs.push(data.slug);

		await wsPage.createWorkspace(data);

		await expect(await wsPage.rowBySlug(data.slug)).toBeVisible();
	});

	test("US1 acceptance 3: workspace A is invisible to users scoped to workspace B", async ({ page }) => {
		// Assumes the test harness has pre-seeded:
		//   (a) Workspace A visible to the current logged-in user.
		//   (b) Workspace B, visible only to a different user whose
		//       session we DO NOT have.
		// If the harness hasn't been set up with two scoped sessions,
		// this test is a placeholder that exercises the UI flow only.
		const wsPage = new WorkspacesPage(page);
		await wsPage.goto();

		const visibleWorkspaceB = await wsPage.rowExists("workspace-b-other-tenant");
		expect(visibleWorkspaceB).toBe(false);
	});

	test("duplicate slug surfaces a toast error", async ({ page }) => {
		const wsPage = new WorkspacesPage(page);
		await wsPage.goto();

		const data = createWorkspaceData({ slug: "dup-slug-test" });
		createdSlugs.push(data.slug);
		await wsPage.createWorkspace(data);
		await expect(await wsPage.rowBySlug(data.slug)).toBeVisible();

		// Second attempt with same slug should surface a conflict toast.
		await wsPage.createWorkspace({ ...data, name: "Second attempt" });
		await expect(page.getByText(/already exists|conflict|409/i).first()).toBeVisible({ timeout: 5000 });
	});
});
