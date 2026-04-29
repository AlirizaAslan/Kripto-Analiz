import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const response = await fetch('/api/market');
	const overview = await response.json();

	return {
		overview
	};
};
