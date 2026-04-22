import { redirect } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { PageServerLoad } from './$types';

// Handles the email verification link: /auth/verify-email?token=...
// The token is validated server-side via INTERNAL_URL so it works
// in all environments (dev, Docker, prod) without a reverse proxy.
export const load: PageServerLoad = async ({ url }) => {
	const token = url.searchParams.get('token');
	if (!token) return {};

	const internalBase = env.INTERNAL_URL ?? 'http://localhost:8080';
	const backendUrl = `${internalBase}/api/v1/auth/verify-email?token=${encodeURIComponent(token)}`;

	let redirectPath = '/auth/verify-email?error=token_error';

	try {
		const res = await fetch(backendUrl, { redirect: 'manual' });
		const location = res.headers.get('location');
		if (location) {
			const parsed = new URL(location);
			redirectPath = parsed.pathname + parsed.search;
		}
	} catch {
		// network error — fall through to default error redirect
	}

	throw redirect(302, redirectPath);
};
