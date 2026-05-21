// @ts-nocheck
import { writeFileSync, readFileSync, existsSync, mkdirSync } from 'fs';
import { join } from 'path';

const DATA_DIR = join(process.cwd(), '.data');
const STORE_FILE = join(DATA_DIR, 'predictions.json');

export type StoredPrediction = {
	symbol: string;
	market: string;
	targetCandleStart: string;
	predictedDirection: 'up' | 'down' | 'neutral';
	realizedDirection: 'up' | 'down' | 'neutral' | '';
	confidenceScore: number;
	wasCorrect: boolean;
	tradeAllowed: boolean;
	tradeAction: string;
	tradeFilterReason: string;
	consensusActive: boolean;
	consensusDirection: string;
	consensusStrength: string;
	predictionHourUTC: number;
	isPending: boolean;
	volatilityScore: number;
};

type StoreData = {
	predictions: StoredPrediction[];
};

let cache: StoreData | null = null;

function ensureDir() {
	if (!existsSync(DATA_DIR)) {
		mkdirSync(DATA_DIR, { recursive: true });
	}
}

function loadStore(): StoreData {
	if (cache) return cache;
	ensureDir();
	if (existsSync(STORE_FILE)) {
		try {
			const raw = readFileSync(STORE_FILE, 'utf-8');
			cache = JSON.parse(raw) as StoreData;
			return cache;
		} catch {
			cache = { predictions: [] };
			return cache;
		}
	}
	cache = { predictions: [] };
	return cache;
}

function saveStore(data: StoreData) {
	ensureDir();
	cache = data;
	try {
		writeFileSync(STORE_FILE, JSON.stringify(data), 'utf-8');
	} catch {
		// Local mock persistence is best-effort only.
	}
}

export function recordPrediction(pred: StoredPrediction) {
	const store = loadStore();
	const existing = store.predictions.findIndex(
		(item) => item.symbol === pred.symbol && item.targetCandleStart === pred.targetCandleStart
	);
	if (existing >= 0) {
		if (pred.realizedDirection && !store.predictions[existing].isPending) {
			store.predictions[existing].realizedDirection = pred.realizedDirection;
			store.predictions[existing].wasCorrect =
				pred.predictedDirection === pred.realizedDirection;
			store.predictions[existing].isPending = false;
		}
	} else {
		store.predictions.push(pred);
	}
	if (store.predictions.length > 5000) {
		store.predictions = store.predictions.slice(-5000);
	}
	saveStore(store);
}

export function recordPredictionsFromOverview(
	summaries: Array<{
		asset: { symbol: string; market: string; volatilityScore: number };
		latestPrediction: {
			predictedDirection: 'up' | 'down' | 'neutral';
			targetCandleStart: string;
			confidenceScore: number;
			tradeAllowed: boolean;
			tradeAction: string;
			tradeFilterReason: string;
			consensusActive: boolean;
			consensusDirection: string;
			consensusStrength: string;
		};
	}>
) {
	for (const summary of summaries) {
		const pred = summary.latestPrediction;
		recordPrediction({
			symbol: summary.asset.symbol,
			market: summary.asset.market,
			targetCandleStart: pred.targetCandleStart,
			predictedDirection: pred.predictedDirection,
			realizedDirection: '',
			confidenceScore: pred.confidenceScore,
			wasCorrect: false,
			tradeAllowed: pred.tradeAllowed,
			tradeAction: pred.tradeAction,
			tradeFilterReason: pred.tradeFilterReason,
			consensusActive: pred.consensusActive,
			consensusDirection: pred.consensusDirection,
			consensusStrength: pred.consensusStrength,
			predictionHourUTC: new Date(pred.targetCandleStart).getUTCHours(),
			isPending: true,
			volatilityScore: summary.asset.volatilityScore
		});
	}
}

export function resolvePredictions(
	symbol: string,
	candles: Array<{ timestamp: string; open: number; close: number }>
) {
	const store = loadStore();
	let changed = false;
	for (const candle of candles) {
		const realized: 'up' | 'down' | 'neutral' =
			candle.close > candle.open ? 'up' : candle.close < candle.open ? 'down' : 'neutral';
		const pred = store.predictions.find(
			(item) => item.symbol === symbol && item.targetCandleStart === candle.timestamp && item.isPending
		);
		if (!pred) continue;
		pred.realizedDirection = realized;
		pred.wasCorrect = pred.predictedDirection === realized;
		pred.isPending = false;
		changed = true;
	}
	if (changed) {
		saveStore(store);
	}
}
