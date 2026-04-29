import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { Market } from '$lib/types';

function parseMarket(value: string): Market | null {
	return value === 'us_equities' || value === 'bist' || value === 'crypto' ? value : null;
}

export const load: PageLoad = async ({ fetch, params }) => {
	const market = parseMarket(params.market);
	if (!market) {
		throw error(400, 'Desteklenmeyen piyasa');
	}

	const response = await fetch(`/api/markets/${market}/symbols/${params.symbol}`);
	if (!response.ok) {
		throw error(response.status, 'Varlik bulunamadi');
	}

	return await response.json();
};
