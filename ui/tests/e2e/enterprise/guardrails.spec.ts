// Playwright E2E — guardrails runtime (spec 016 T015).
//
// Preconditions (for CI):
//   - Enterprise build live on baseURL with guardrails-runtime plugin
//     registered (server.go wiring).
//   - At least one provider configured (regex needs none) + a rule
//     with trigger=input + action=block targeting a stable pattern.
//
// This spec follows the same path as other /ui/tests/e2e/enterprise/*
// files — scaffolded against the infra pending T031 carryover from
// spec 001. Once the Playwright runner lands for enterprise tests,
// these execute as-is.

import { expect, test } from "@playwright/test";

test.describe("Spec 016 — Guardrails Runtime", () => {
	test("admin can create a regex block rule", async ({ page }) => {
		await page.goto("/workspace/guardrails/configuration");
		await expect(page.getByTestId("guardrails-rules-view")).toBeVisible();

		await page.getByRole("button", { name: /new rule/i }).click();
		await page.getByLabel(/^name$/i).fill("e2e-ccn-block");
		// Regex-only rule: leave provider empty.
		await page.getByLabel(/pattern/i).fill("\\b\\d{16}\\b");
		await page.getByLabel(/trigger/i).click();
		await page.getByRole("option", { name: /^input$/i }).click();
		await page.getByLabel(/action/i).click();
		await page.getByRole("option", { name: /^block$/i }).click();
		await page.getByRole("button", { name: /^save$/i }).click();

		await expect(page.getByText("e2e-ccn-block")).toBeVisible();
	});

	test("inference with matching prompt is blocked with 451", async ({ request }) => {
		// Assumes the prior test seeded the rule OR a baseline fixture
		// created it. Uses a virtual key that permits the target model.
		const resp = await request.post("/openai/v1/chat/completions", {
			headers: {
				"Content-Type": "application/json",
				"Authorization": "Bearer " + (process.env.BIFROST_VK ?? "sk-bf-test"),
			},
			data: {
				model: "gpt-4o-mini",
				messages: [
					{ role: "user", content: "card number 4111111111111111" },
				],
			},
			failOnStatusCode: false,
		});
		expect(resp.status()).toBe(451);
		const body = await resp.json();
		const bodyStr = JSON.stringify(body);
		expect(bodyStr).toContain("guardrail");
	});

	test("inference without matching prompt passes through", async ({ request }) => {
		const resp = await request.post("/openai/v1/chat/completions", {
			headers: {
				"Content-Type": "application/json",
				"Authorization": "Bearer " + (process.env.BIFROST_VK ?? "sk-bf-test"),
			},
			data: {
				model: "gpt-4o-mini",
				messages: [
					{ role: "user", content: "say hello" },
				],
			},
			failOnStatusCode: false,
		});
		// Either a real provider response (200) or a provider-auth
		// error (401/403) — but NEVER 451, which would indicate a
		// guardrail false positive.
		expect(resp.status()).not.toBe(451);
	});

	test("audit log captures the block", async ({ page }) => {
		await page.goto("/workspace/audit-logs");
		await expect(page.getByTestId("audit-logs-view")).toBeVisible();
		await page.getByTestId("audit-filter-action").fill("guardrail.block");
		await page.getByTestId("audit-apply-filters").click();
		// At least one row — the block we issued in the earlier test
		// (or any prior run).
		await expect(page.getByRole("cell", { name: /guardrail\.block/i }).first()).toBeVisible();
	});
});
