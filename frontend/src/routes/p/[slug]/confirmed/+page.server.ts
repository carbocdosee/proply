import type { PageServerLoad } from './$types';
import { error } from '@sveltejs/kit';

const INTERNAL_API_URL = process.env.INTERNAL_URL ?? 'http://localhost:8080';

export const load: PageServerLoad = async ({ params, fetch }) => {
	const { slug } = params;

	const res = await fetch(`${INTERNAL_API_URL}/api/v1/public/proposals/${slug}`);

	// For revoked/not-found slugs the confirmed page still makes sense to show
	// (client already approved before revocation), so be lenient with errors.
	if (!res.ok) {
		return {
			agencyName: '',
			proposalTitle: '',
			approvedAt: null as string | null,
		};
	}

	const proposal = await res.json();

	if (proposal.status !== 'approved') {
		throw error(404, 'Proposal not found');
	}

	return {
		agencyName: (proposal.agency_name as string) ?? '',
		proposalTitle: (proposal.title as string) ?? '',
		approvedAt: (proposal.approved_at as string | null) ?? null,
	};
};
