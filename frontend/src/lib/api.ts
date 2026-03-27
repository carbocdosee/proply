/**
 * API client for Go backend.
 * All requests go through SvelteKit server-side routes to avoid CORS and token leakage.
 */

const API_BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

export interface ApiError {
	code: string;
	message?: string;
}

export class HttpError extends Error {
	constructor(
		public status: number,
		public code: string
	) {
		super(`HTTP ${status}: ${code}`);
	}
}

async function request<T>(
	path: string,
	options: RequestInit & { token?: string } = {}
): Promise<T> {
	const { token, ...fetchOptions } = options;

	const headers = new Headers(fetchOptions.headers);
	headers.set('Content-Type', 'application/json');
	if (token) {
		headers.set('Authorization', `Bearer ${token}`);
	}

	const res = await fetch(`${API_BASE}${path}`, {
		...fetchOptions,
		headers,
		credentials: 'include'
	});

	if (!res.ok) {
		let code = 'UNKNOWN_ERROR';
		try {
			const body = await res.json();
			code = body.code ?? code;
		} catch {
			// ignore parse errors
		}
		throw new HttpError(res.status, code);
	}

	if (res.status === 204) {
		return undefined as T;
	}

	return res.json() as Promise<T>;
}

export interface AuthResponse {
	access_token: string;
	email_verified: boolean;
}

// Auth
export const auth = {
	register: (email: string, password: string, name: string) =>
		request<AuthResponse>('/api/v1/auth/register', {
			method: 'POST',
			body: JSON.stringify({ email, password, name })
		}),

	login: (email: string, password: string) =>
		request<AuthResponse>('/api/v1/auth/login', {
			method: 'POST',
			body: JSON.stringify({ email, password })
		}),

	refresh: () =>
		request<AuthResponse>('/api/v1/auth/refresh', { method: 'POST' }),

	logout: () => request<void>('/api/v1/auth/logout', { method: 'POST' }),

	me: (token: string) =>
		request<User>('/api/v1/auth/me', { token }),

	// Magic link (passwordless)
	sendMagicLink: (email: string) =>
		request<{ message: string }>('/api/v1/auth/magic-link', {
			method: 'POST',
			body: JSON.stringify({ email })
		}),

	// Resend verification email (requires auth)
	resendVerification: (token: string) =>
		request<{ message: string }>('/api/v1/auth/resend-verification', {
			method: 'POST',
			token
		})
};

// Proposals
export const proposals = {
	list: (token: string, params?: Record<string, string>) => {
		const qs = params ? '?' + new URLSearchParams(params).toString() : '';
		return request<ProposalListResult>(`/api/v1/proposals${qs}`, { token });
	},

	create: (token: string, data: { title?: string; client_name?: string; template_id?: string }) =>
		request<{ id: string }>('/api/v1/proposals', {
			method: 'POST',
			token,
			body: JSON.stringify(data)
		}),

	get: (token: string, id: string) =>
		request<Proposal>(`/api/v1/proposals/${id}`, { token }),

	update: (token: string, id: string, data: Partial<Proposal>) =>
		request<{ updated_at: string }>(`/api/v1/proposals/${id}`, {
			method: 'PATCH',
			token,
			body: JSON.stringify(data)
		}),

	publish: (token: string, id: string) =>
		request<{ slug: string }>(`/api/v1/proposals/${id}/publish`, { method: 'POST', token }),

	revoke: (token: string, id: string) =>
		request<{ slug_active: false }>(`/api/v1/proposals/${id}/revoke`, { method: 'POST', token }),

	duplicate: (token: string, id: string) =>
		request<{ id: string }>(`/api/v1/proposals/${id}/duplicate`, { method: 'POST', token }),

	delete: (token: string, id: string) =>
		request<void>(`/api/v1/proposals/${id}`, { method: 'DELETE', token }),

	getAnalytics: (token: string, id: string) =>
		request<Analytics>(`/api/v1/proposals/${id}/analytics`, { token })
};

// Public (no auth)
export const publicApi = {
	getProposal: (slug: string, password?: string) => {
		const headers: Record<string, string> = {};
		if (password) headers['X-Proposal-Password'] = password;
		return request<PublicProposal>(`/api/v1/public/proposals/${slug}`, { headers });
	},

	approve: (slug: string, clientEmail: string) =>
		request<{ approved_at: string }>(`/api/v1/public/proposals/${slug}/approve`, {
			method: 'POST',
			body: JSON.stringify({ client_email: clientEmail })
		})
};

// Types
export interface User {
	id: string;
	email: string;
	name: string;
	plan: 'free' | 'pro' | 'team';
	language: 'en' | 'ru';
	logo_url?: string;
	primary_color: string;
	accent_color: string;
	hide_proply_footer: boolean;
	email_verified_at: string | null;
	created_at: string;
}

export interface Block {
	id: string;
	type: 'text' | 'price_table' | 'case_study' | 'team_member' | 'terms';
	order: number;
	data: Record<string, unknown>;
}

export interface Proposal {
	id: string;
	title: string;
	client_name: string;
	client_email?: string;
	status: 'draft' | 'sent' | 'opened' | 'approved' | 'rejected';
	slug?: string;
	slug_active: boolean;
	blocks: Block[];
	template_id?: string;
	open_count: number;
	first_opened_at?: string;
	last_opened_at?: string;
	approved_at?: string;
	created_at: string;
	updated_at: string;
}

export interface PublicProposal {
	title: string;
	client_name: string;
	agency_name: string;
	logo_url?: string;
	primary_color: string;
	accent_color: string;
	hide_proply_footer: boolean;
	language: string;
	blocks: Block[];
	status: string;
	approved_at?: string;
	password_protected: boolean;
}

export interface ProposalListResult {
	items: Proposal[];
	total: number;
	page: number;
	per_page: number;
	plan_usage: { used: number; limit: number | null };
}

export interface Analytics {
	open_count: number;
	first_opened_at?: string;
	last_opened_at?: string;
	total_duration_sec: number;
	block_stats: Array<{
		block_id: string;
		block_type: string;
		order: number;
		duration_sec: number;
	}>;
	events: Array<{
		id: string;
		event_type: string;
		country?: string;
		user_agent: string;
		created_at: string;
		duration_ms?: number;
	}>;
	plan_gate: boolean;
}
