import { json } from '@sveltejs/kit';
import { getMarketOverview } from '$lib/server/market-engine';

export async function GET() {
	const overview = await getMarketOverview();
	return json(overview, {
		headers: {
			'cache-control': 'no-store'
		}
	});
}
