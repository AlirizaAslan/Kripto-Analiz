import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';

export async function GET() {
	const baseURL = env.PULSEALPHA_API_BASE_URL || 'http://127.0.0.1:8080';
	try {
		const response = await fetch(`${baseURL}/api/statistics`);
		if (!response.ok) {
			return json({ error: 'Statistics not found' }, { status: response.status });
		}
		const data = await response.json();
		const normalized =
			Array.isArray(data)
				? { generatedAt: new Date().toISOString(), items: data }
				: {
						generatedAt: data?.generatedAt ?? new Date().toISOString(),
						items: Array.isArray(data?.items) ? data.items : []
					};
		return json(normalized, {
			headers: { 'cache-control': 'no-store' }
		});
	} catch {
		return json({ error: 'Failed to fetch statistics' }, { status: 500 });
	}
}
