/**
 * Dashboard (React + Vite SPA) auth adapter.
 *
 * Pure SPA — no SSR. Default adapter covers everything. Importing this module
 * registers the adapter; dashboard main.tsx should `import '@orinforum/api-client/adapters/dashboard'`.
 */
import { type AuthAdapter, defaultAdapter, registerAdapter } from '../../adapter';

const dashboardAdapter: AuthAdapter = {
	...defaultAdapter,
	isSSR: () => false,
	fetchCredentials: () => 'include',
};

registerAdapter(dashboardAdapter);

export { dashboardAdapter };
