import type { PageServerLoad } from './$types';
import { redirect } from '@sveltejs/kit';

const INTERNAL_API_URL = process.env.INTERNAL_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ url, fetch }) => {
	const token = url.searchParams.get('token') ?? '';

	if (!token) {
		throw redirect(302, '/auth/magic-link?error=missing_token');
	}

	const res = await fetch(
		`${INTERNAL_API_URL}/api/v1/auth/magic-link/verify?token=${encodeURIComponent(token)}`,
		{ redirect: 'manual' }
	);

	const location = res.headers.get('location') ?? '';

	if (location) {
		// Backend returns absolute URL — extract pathname + search for same-origin redirect
		try {
			const dest = new URL(location);
			throw redirect(302, dest.pathname + dest.search);
		} catch (e) {
			if (e instanceof Response) throw e;
			throw redirect(302, location);
		}
	}

	throw redirect(302, '/auth/magic-link?error=invalid_or_expired');
};
