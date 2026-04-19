// Playwright page-object for the enterprise workspaces UI.
//
// Mirrors the pattern in tests/e2e/features/governance/pages/. The
// isolation spec (workspaces-isolation.spec.ts) drives this page
// object to walk the US1 acceptance flow.

import type { Locator, Page } from "@playwright/test";

export class WorkspacesPage {
	readonly page: Page;
	readonly newBtn: Locator;
	readonly table: Locator;
	readonly emptyAddBtn: Locator;
	readonly nameInput: Locator;
	readonly slugInput: Locator;
	readonly descInput: Locator;
	readonly submitBtn: Locator;
	readonly cancelBtn: Locator;

	constructor(page: Page) {
		this.page = page;
		this.newBtn = page.getByTestId("workspace-button-new");
		this.table = page.getByTestId("workspace-list-table");
		this.emptyAddBtn = page.getByTestId("workspace-button-add");
		this.nameInput = page.getByTestId("workspace-input-name");
		this.slugInput = page.getByTestId("workspace-input-slug");
		this.descInput = page.getByTestId("workspace-input-description");
		this.submitBtn = page.getByTestId("workspace-button-submit");
		this.cancelBtn = page.getByTestId("workspace-button-cancel");
	}

	async goto() {
		await this.page.goto("/workspace/workspaces");
	}

	async openCreateDialog() {
		const newVisible = await this.newBtn.isVisible().catch(() => false);
		if (newVisible) {
			await this.newBtn.click();
		} else {
			await this.emptyAddBtn.click();
		}
	}

	async createWorkspace({ name, slug, description }: { name: string; slug: string; description?: string }) {
		await this.openCreateDialog();
		await this.nameInput.fill(name);
		await this.slugInput.fill(slug);
		if (description) await this.descInput.fill(description);
		await this.submitBtn.click();
	}

	async rowBySlug(slug: string): Promise<Locator> {
		return this.page.getByTestId(`workspace-list-row-${slug}`);
	}

	async rowExists(slug: string): Promise<boolean> {
		const row = await this.rowBySlug(slug);
		return await row.isVisible().catch(() => false);
	}

	async openDetail(slug: string) {
		const row = await this.rowBySlug(slug);
		await row.getByRole("link").first().click();
	}

	async deleteWorkspace(slug: string) {
		this.page.on("dialog", (d) => d.accept());
		await this.page.getByTestId(`workspace-button-delete-${slug}`).click();
	}
}
