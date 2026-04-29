import { error, json } from '@sveltejs/kit';
import { getAssetDetail } from '$lib/server/market-engine';
import type { Market } from '$lib/types';

function parseMarket(value: string): Market | null {
	return value === 'us_equities' || value === 'bist' || value === 'crypto' ? value : null;
}

export async function GET({ params }) {
	const market = parseMarket(params.market);
	if (!market) {
		throw error(400, 'Desteklenmeyen piyasa');
	}

	const detail = await getAssetDetail(market, params.symbol);
	if (!detail) {
		throw error(503, 'Canli Binance verisi su anda alinamiyor');
	}

	return json(detail, {
		headers: {
			'cache-control': 'no-store'
		}
	});
}
