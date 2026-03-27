import { writable, derived } from 'svelte/store';
import type { User } from '$lib/api';

interface AuthState {
	user: User | null;
	accessToken: string | null;
	emailVerified: boolean;
	loading: boolean;
}

function createAuthStore() {
	const { subscribe, set, update } = writable<AuthState>({
		user: null,
		accessToken: null,
		emailVerified: false,
		loading: true
	});

	return {
		subscribe,

		setUser(user: User, accessToken: string, emailVerified = false) {
			update((s) => ({ ...s, user, accessToken, emailVerified, loading: false }));
		},

		setToken(accessToken: string, emailVerified = false) {
			update((s) => ({ ...s, accessToken, emailVerified }));
		},

		markEmailVerified() {
			update((s) => ({ ...s, emailVerified: true }));
		},

		logout() {
			set({ user: null, accessToken: null, emailVerified: false, loading: false });
		},

		setLoading(loading: boolean) {
			update((s) => ({ ...s, loading }));
		}
	};
}

export const authStore = createAuthStore();

export const isAuthenticated = derived(authStore, ($auth) => !!$auth.accessToken && !!$auth.user);
export const currentPlan = derived(authStore, ($auth) => $auth.user?.plan ?? 'free');
export const isEmailVerified = derived(authStore, ($auth) => $auth.emailVerified);
