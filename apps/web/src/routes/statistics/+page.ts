import type { PageLoad } from './$types';
import type { MarketOverview, StatisticsOverview } from '$lib/types';

type StatisticsPageData = {
	overview: MarketOverview;
	statistics: StatisticsOverview;
	statisticsMessage: string;
};

async function safeJSON<T>(response: Response): Promise<T | null> {
	if (!response.ok) return null;
	try {
		return (await response.json()) as T;
	} catch {
		return null;
	}
}

export const load: PageLoad = async ({ fetch }) => {
	const overviewResponse = await fetch('/api/market');
	const overview = (await overviewResponse.json()) as MarketOverview;

	let statisticsMessage = '';

	const statisticsResponse = await fetch('/api/statistics');
	const statistics = await safeJSON<StatisticsOverview>(statisticsResponse);
	const statisticsItems = statistics?.items ?? [];
	if (!statisticsResponse.ok) {
		statisticsMessage = 'Kalici istatistikler gecici olarak alinamadi.';
	} else if (!statistics?.items?.length) {
		statisticsMessage = 'Kalici istatistik kaydi henuz olusmadi.';
	}

	const result: StatisticsPageData = {
		overview,
		statistics: {
			generatedAt: statistics?.generatedAt ?? new Date().toISOString(),
			items: statisticsItems
		},
		statisticsMessage
	};

	return result;
};
