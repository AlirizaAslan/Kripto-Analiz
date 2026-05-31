import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { AssetDetail, Market, SymbolStatistics } from '$lib/types';

function parseMarket(value: string): Market | null {
	return value === 'us_equities' || value === 'bist' || value === 'crypto' ? value : null;
}

export const load: PageLoad = async ({ fetch, params, url }) => {
	const market = parseMarket(params.market);
	if (!market) {
		throw error(400, 'Desteklenmeyen piyasa');
	}

	const focusTimestamp = url.searchParams.get('focus')?.trim() || null;

	const [detailResponse, statisticsResponse] = await Promise.all([
		fetch(`/api/markets/${market}/symbols/${params.symbol}`),
		fetch(`/api/markets/${market}/symbols/${params.symbol}/statistics`)
	]);

	if (!detailResponse.ok) {
		throw error(detailResponse.status, 'Varlik bulunamadi');
	}

	const detail = (await detailResponse.json()) as AssetDetail;
	let statistics: SymbolStatistics | null = null;

	if (statisticsResponse.ok) {
		statistics = (await statisticsResponse.json()) as SymbolStatistics;
	}

	return { detail, statistics, focusTimestamp };
};
