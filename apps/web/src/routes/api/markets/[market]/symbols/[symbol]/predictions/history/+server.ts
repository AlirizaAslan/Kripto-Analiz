import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

export async function GET({ params }) {
	const { market, symbol } = params;
	const baseURL = env.PULSEALPHA_API_BASE_URL || 'http://127.0.0.1:8080';
	const response = await fetch(`${baseURL}/api/markets/${market}/symbols/${symbol}/predictions/history`);
	if (!response.ok) {
		return json({ error: 'Prediction history not found' }, { status: response.status });
	}
	const data = await response.json();
	return json(data, {
		headers: { 'cache-control': 'no-store' }
	});
}
