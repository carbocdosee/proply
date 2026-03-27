import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';

const INTERNAL_API_URL = process.env.INTERNAL_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ params, request, fetch }) => {
	const { slug } = params;

	// Server-side tracking: fire-and-forget before fetching proposal data
	// This is a server-to-server call — bypasses any client-side ad blockers
	const ip =
		request.headers.get('CF-Connecting-IP') ??
		request.headers.get('X-Forwarded-For')?.split(',')[0]?.trim() ??
		'unknown';

	const cfCountry = request.headers.get('CF-IPCountry') ?? '';
	const userAgent = request.headers.get('User-Agent') ?? '';

	// Non-blocking track (do not await — don't delay page render)
	fetch(`${INTERNAL_API_URL}/api/v1/internal/track/open`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ slug, ip, user_agent: userAgent, cf_country: cfCountry })
	}).catch(() => {
		// Silently ignore tracking errors — proposal render must not be blocked
	});

	// Fetch proposal data
	const res = await fetch(`${INTERNAL_API_URL}/api/v1/public/proposals/${slug}`, {
		headers: { 'Content-Type': 'application/json' }
	});

	if (res.status === 401) {
		const body = await res.json();
		if (body.code === 'PASSWORD_REQUIRED') {
			return { proposal: null, passwordRequired: true, slug };
		}
	}

	if (res.status === 404) {
		throw error(404, 'Proposal not found');
	}

	if (!res.ok) {
		throw error(500, 'Failed to load proposal');
	}

	const proposal = await res.json();
	return { proposal, passwordRequired: false, slug };
};
