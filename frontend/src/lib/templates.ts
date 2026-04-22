/**
 * Static template catalog — mirrors backend service/templates.go.
 * Used by the template picker modal to render previews without an API call.
 */

export interface TemplateMeta {
	id: string;
	name: string;
	description: string;
	blockTypes: string[]; // ordered list of block types for preview badges
}

export interface TemplateCard extends TemplateMeta {
	icon: string;      // emoji icon
	color: string;     // Tailwind bg color class for the icon container
	accentText: string; // Tailwind text color class
}

// Matches the IDs in backend service/templates.go
export const TEMPLATES: TemplateCard[] = [
	{
		id: 'web',
		name: 'Web Development',
		description: 'Website or web-app project with scope, pricing, and portfolio.',
		blockTypes: ['text', 'price_table', 'case_study', 'terms'],
		icon: '🌐',
		color: 'bg-indigo-50',
		accentText: 'text-indigo-600'
	},
	{
		id: 'seo',
		name: 'SEO Optimization',
		description: 'Search engine optimization with deliverables and timeline.',
		blockTypes: ['text', 'price_table', 'terms'],
		icon: '📈',
		color: 'bg-green-50',
		accentText: 'text-green-600'
	},
	{
		id: 'smm',
		name: 'Social Media',
		description: 'Content strategy, community management, and paid social.',
		blockTypes: ['text', 'price_table', 'terms'],
		icon: '📣',
		color: 'bg-pink-50',
		accentText: 'text-pink-600'
	},
	{
		id: 'design',
		name: 'Design & Branding',
		description: 'Identity design, UI/UX, or brand refresh with portfolio examples.',
		blockTypes: ['text', 'price_table', 'case_study', 'terms'],
		icon: '🎨',
		color: 'bg-orange-50',
		accentText: 'text-orange-600'
	},
	{
		id: 'consulting',
		name: 'Consulting',
		description: 'Strategy or advisory engagement with team bios and deliverables.',
		blockTypes: ['text', 'price_table', 'team_member', 'terms'],
		icon: '💼',
		color: 'bg-purple-50',
		accentText: 'text-purple-600'
	}
];

// Human-readable labels for block type badges
export const BLOCK_TYPE_LABELS: Record<string, string> = {
	text: 'Intro text',
	price_table: 'Price table',
	case_study: 'Case study',
	team_member: 'Team member',
	terms: 'Terms'
};
