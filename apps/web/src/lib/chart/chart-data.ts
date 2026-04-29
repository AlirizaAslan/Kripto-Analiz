import type { Candle } from '$lib/types';
import type { ChartTimeframe } from './pro-chart-theme';

export type ChartPoint = Candle & {
	time: number;
	change: number;
	range: number;
	direction: 'up' | 'down';
};

export function toUnixTime(timestamp: string) {
	const time = Math.floor(new Date(timestamp).getTime() / 1000);
	return Number.isFinite(time) ? time : 0;
}

export function normalizeCandles(candles: Candle[]): ChartPoint[] {
	return candles
		.map((candle) => {
			const time = toUnixTime(candle.timestamp);
			const change = candle.close - candle.open;
			const direction: 'up' | 'down' = candle.close >= candle.open ? 'up' : 'down';
			return {
				...candle,
				time,
				volume: Number.isFinite(candle.volume) ? candle.volume : 0,
				change,
				range: candle.high - candle.low,
				direction
			};
		})
		.filter((candle) => candle.time > 0)
		.sort((left, right) => left.time - right.time);
}

export function movingAverage(points: ChartPoint[], period: number, field: 'close' | 'volume' = 'close') {
	const output: Array<{ time: number; value: number }> = [];
	let sum = 0;
	for (let index = 0; index < points.length; index += 1) {
		sum += points[index][field];
		if (index >= period) {
			sum -= points[index - period][field];
		}
		if (index >= period - 1) {
			output.push({
				time: points[index].time,
				value: sum / period
			});
		}
	}
	return output;
}

export function formatTimeframeLabel(timeframe: ChartTimeframe) {
	return timeframe.toUpperCase();
}
