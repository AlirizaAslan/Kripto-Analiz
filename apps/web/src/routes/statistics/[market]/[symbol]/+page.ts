import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { AssetDetail, Market, SymbolStatistics } from '$lib/types';

type StatisticsDetailPageData = {
	detail: AssetDetail;
	statistics: SymbolStatistics;
};

function parseMarket(value: string): Market | null {
	return value === 'us_equities' || value === 'bist' || value === 'crypto' ? value : null;
}

export const load: PageLoad = async ({ fetch, params }) => {
	const market = parseMarket(params.market);
	if (!market) {
		throw error(400, 'Desteklenmeyen piyasa');
	}

	const [detailResponse, statisticsResponse] = await Promise.all([
		fetch(`/api/markets/${market}/symbols/${params.symbol}`),
		fetch(`/api/markets/${market}/symbols/${params.symbol}/statistics`)
	]);

	if (!detailResponse.ok) {
		throw error(detailResponse.status, 'Sembol detayi alinamadi');
	}

	if (!statisticsResponse.ok) {
		throw error(statisticsResponse.status, 'Kalici istatistik alinamadi');
	}

	return {
		detail: (await detailResponse.json()) as AssetDetail,
		statistics: (await statisticsResponse.json()) as SymbolStatistics
	} satisfies StatisticsDetailPageData;
};
