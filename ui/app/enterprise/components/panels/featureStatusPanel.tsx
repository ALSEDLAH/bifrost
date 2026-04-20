// FeatureStatusPanel — honest status panel for enterprise stubs whose
// audit verdict was "descope → panel" (spec 002, US2, FR-003 / FR-004).
//
// Replaces ContactUsView INSIDE the enterprise build only. ContactUsView
// itself is untouched and remains the correct component for the OSS
// fallback build where the commercial license is genuinely the gate.
//
// Contract: specs/002-expose-hidden-enterprise-stubs/contracts/feature-status-panel.md
// Data model: specs/002-expose-hidden-enterprise-stubs/data-model.md

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { Link } from "@tanstack/react-router";
import {
	Archive,
	ArrowRight,
	Clock,
	ExternalLink,
	FileText,
	SplitSquareHorizontal,
	type LucideIcon,
} from "lucide-react";
import { type ComponentType, useMemo } from "react";

export type FeatureStatusLabel =
	| "descoped"
	| "needs-own-spec"
	| "upstream-partial"
	| "pending-implementation";

export interface FeatureStatusPanelProps {
	title: string;
	description: string;
	status: FeatureStatusLabel;
	trackingLink: {
		href: string;
		label: string;
	};
	alternativeRoute?: {
		href: string;
		label: string;
	};
	icon?: ComponentType<{ className?: string }>;
}

const STATUS_CONFIG: Record<
	FeatureStatusLabel,
	{ label: string; icon: LucideIcon; badgeVariant: "default" | "secondary" | "outline" | "destructive" }
> = {
	descoped: { label: "Descoped", icon: Archive, badgeVariant: "secondary" },
	"needs-own-spec": { label: "Needs own spec", icon: FileText, badgeVariant: "outline" },
	"upstream-partial": { label: "Upstream partial", icon: SplitSquareHorizontal, badgeVariant: "outline" },
	"pending-implementation": { label: "Pending implementation", icon: Clock, badgeVariant: "default" },
};

function isExternalHref(href: string): boolean {
	return /^https?:\/\//.test(href);
}

function isSpecHref(href: string): boolean {
	return href.startsWith("/specs/") || href.startsWith("specs/");
}

export default function FeatureStatusPanel({
	title,
	description,
	status,
	trackingLink,
	alternativeRoute,
	icon,
}: FeatureStatusPanelProps) {
	const cfg = STATUS_CONFIG[status];
	const HeaderIcon = icon ?? cfg.icon;
	const headingId = useMemo(
		() => `feature-status-${title.replace(/\s+/g, "-").toLowerCase()}`,
		[title],
	);

	return (
		<div
			role="region"
			aria-labelledby={headingId}
			data-testid="feature-status-panel"
			className={cn(
				"mx-auto my-8 flex w-full max-w-2xl flex-col gap-4 rounded-lg border p-8",
				"bg-card text-card-foreground",
			)}
		>
			<div className="flex items-start gap-4">
				<HeaderIcon className="text-muted-foreground mt-1 h-14 w-14 shrink-0" strokeWidth={1.25} />
				<div className="flex flex-col gap-2">
					<h2 id={headingId} className="text-xl font-semibold tracking-tight">
						{title}
					</h2>
					<div>
						<Badge
							variant={cfg.badgeVariant}
							aria-label={`Status: ${cfg.label}`}
						>
							{cfg.label}
						</Badge>
					</div>
				</div>
			</div>

			<p className="text-muted-foreground text-sm leading-relaxed">{description}</p>

			<div className="flex flex-wrap gap-2 pt-2">
				{isExternalHref(trackingLink.href) || isSpecHref(trackingLink.href) ? (
					<Button asChild variant="outline" size="sm">
						<a
							href={trackingLink.href}
							target={isExternalHref(trackingLink.href) ? "_blank" : undefined}
							rel={isExternalHref(trackingLink.href) ? "noopener noreferrer" : undefined}
							data-testid="feature-status-panel-tracking-link"
						>
							{trackingLink.label}
							<ExternalLink className="ml-2 h-3 w-3" />
						</a>
					</Button>
				) : (
					<Button asChild variant="outline" size="sm">
						<Link
							to={trackingLink.href}
							data-testid="feature-status-panel-tracking-link"
						>
							{trackingLink.label}
							<ArrowRight className="ml-2 h-3 w-3" />
						</Link>
					</Button>
				)}

				{alternativeRoute && (
					<Button asChild variant="default" size="sm">
						<Link
							to={alternativeRoute.href}
							data-testid="feature-status-panel-alternative-route"
						>
							See instead: {alternativeRoute.label}
							<ArrowRight className="ml-2 h-3 w-3" />
						</Link>
					</Button>
				)}
			</div>
		</div>
	);
}
