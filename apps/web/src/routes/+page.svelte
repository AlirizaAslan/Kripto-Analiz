<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import type { AssetSummary, MarketOverview, Candle } from '$lib/types';

	export let data: { overview: MarketOverview };

	let overview = data.overview;
	let loading = false;
	let lastError = '';
	let refreshHandle: ReturnType<typeof setInterval> | undefined;

	const formatter = new Intl.NumberFormat('tr-TR', {
		style: 'currency',
		currency: 'USD',
		maximumFractionDigits: 2
	});

	$: cryptoSummaries = overview.summaries.filter((item) => item.asset.market === 'crypto');
	$: topSummary = cryptoSummaries[0];

	function formatPercent(value: number) {
		return `${value > 0 ? '+' : ''}${value.toFixed(2)}%`;
	}

	function candleGeometry(candles: Candle[], width: number, height: number) {
		if (!candles.length) return [];
		const highs = candles.map((candle) => candle.high);
		const lows = candles.map((candle) => candle.low);
		const min = Math.min(...lows);
		const max = Math.max(...highs);
		const range = max - min || 1;
		const slot = width / candles.length;
		const bodyWidth = Math.max(slot * 0.58, 4);

		return candles.map((candle, index) => {
			const centerX = index * slot + slot / 2;
			const y = (value: number) => height - ((value - min) / range) * height;
			const openY = y(candle.open);
			const closeY = y(candle.close);
			const highY = y(candle.high);
			const lowY = y(candle.low);
			return {
				x: centerX - bodyWidth / 2,
				width: bodyWidth,
				openY,
				closeY,
				highY,
				lowY,
				bodyY: Math.min(openY, closeY),
				bodyHeight: Math.max(Math.abs(closeY - openY), 2),
				rising: candle.close >= candle.open,
				isLive: candle.isLive
			};
		});
	}

	function gridOffsets(count: number, size: number) {
		if (count <= 1) return [0];
		return Array.from({ length: count }, (_, index) => (size / (count - 1)) * index);
	}

	function directionLabel(value: 'up' | 'down' | 'neutral' | '') {
		if (value === 'up') return 'Yukselecek';
		if (value === 'down') return 'Dusecek';
		if (value === 'neutral') return 'Yatay';
		return 'Bekleniyor';
	}

	function consensusText(summary: AssetSummary) {
		if (summary.latestPrediction.consensusActive) {
			return directionLabel(summary.latestPrediction.consensusDirection);
		}
		return 'Bekleniyor';
	}

	function consensusTradeText(summary: AssetSummary) {
		if (summary.latestPrediction.tradeAction === 'buy') return 'AL';
		if (summary.latestPrediction.tradeAction === 'sell') return 'SAT';
		if (summary.latestPrediction.tradeAction === 'hold') return 'BEKLE';
		return 'Islem yok';
	}

	function consensusAccuracyText(summary: AssetSummary) {
		if (summary.accuracy.consensusSampleSize > 0) {
			return `${Math.round(summary.accuracy.consensusWinRate * 100)}%`;
		}
		return 'Bekleniyor';
	}

	async function refresh() {
		loading = true;
		try {
			const response = await fetch(`/api/market?t=${Date.now()}`, { cache: 'no-store' });
			if (!response.ok) throw new Error('Piyasa ozeti alinamadi');
			overview = (await response.json()) as MarketOverview;
			lastError = '';
		} catch (error) {
			lastError = error instanceof Error ? error.message : 'Canli yenileme basarisiz oldu';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void refresh();
		refreshHandle = setInterval(refresh, 5_000);
	});

	onDestroy(() => {
		if (refreshHandle) clearInterval(refreshHandle);
	});
</script>

<svelte:head>
	<title>PulseAlpha | 1 dakikalik piyasa tahmini</title>
</svelte:head>

<div class="shell">
	<header class="masthead">
		<div class="brand">
			<p class="eyebrow">PulseAlpha / kripto tahmin motoru</p>
			<h1>Canli kripto mumlari ve gorunur model dogrulugu ile yapay zeka analizi.</h1>
			<p class="intro">
				Bu panel kripto sembollerini canli mum onizlemesi, emir defteri baskisi,
				guncel model isabet orani ve acik risk cercevesiyle izler.
			</p>
		</div>

		<div class="meta-card">
			<p class="meta-label">Veri modu</p>
			<strong>{overview.mode}</strong>
			<p>{overview.feedStatus}</p>
			<div class="meta-row">
				<span class:live={overview.mode !== 'heuristic_fallback'} class="pulse"></span>
				{loading ? '1 dakikalik gorunum yenileniyor' : `Guncellendi ${new Date(overview.generatedAt).toLocaleTimeString('tr-TR')}`}
			</div>
		</div>
	</header>

	{#if topSummary}
		<section class="hero-grid">
			<article class="signal-card">
				<div class="section-head">
					<div>
						<p class="eyebrow">En yuksek guvenli sembol</p>
						<h2>{topSummary.asset.symbol} / {topSummary.asset.name}</h2>
					</div>
					<div class="badge">Kripto / {topSummary.asset.nextCandleInterval}</div>
				</div>

				<svg class="hero-chart" viewBox="0 0 420 160" aria-label={`${topSummary.asset.symbol} mini candlestick`}>
					<rect class="chart-bg" x="0" y="0" width="420" height="160"></rect>
					{#each gridOffsets(5, 160) as y}
						<line class="grid-line" x1="0" x2="420" y1={y} y2={y}></line>
					{/each}
					{#each gridOffsets(7, 420) as x}
						<line class="grid-line vertical" x1={x} x2={x} y1="0" y2="160"></line>
					{/each}
					{#each candleGeometry(topSummary.miniCandles, 420, 160) as candle}
						<line class:up={candle.rising} class:down={!candle.rising} class:live={candle.isLive} class="wick" x1={candle.x + candle.width / 2} x2={candle.x + candle.width / 2} y1={candle.highY} y2={candle.lowY}></line>
						<rect class:up={candle.rising} class:down={!candle.rising} class:live={candle.isLive} class="body" x={candle.x} y={candle.bodyY} width={candle.width} height={candle.bodyHeight}></rect>
					{/each}
				</svg>

				<div class="stats-row">
					<div>
						<span>Ortak karar tahmini</span>
						<strong>{consensusText(topSummary)}</strong>
					</div>
					<div>
						<span>Ortak islem</span>
						<strong>{consensusTradeText(topSummary)}</strong>
					</div>
					<div>
						<span>Ortak karar isabeti</span>
						<strong>{topSummary.accuracy.consensusTradeSampleSize > 0 ? `${Math.round(topSummary.accuracy.consensusTradeWinRate * 100)}%` : 'Bekleniyor'}</strong>
					</div>
				</div>

				<p class="analysis">{topSummary.latestPrediction.tradeFilterReason || topSummary.latestPrediction.consensusSummary || 'Iki model ayni yone bakarsa ortak islem sinyali burada gosterilir.'}</p>
				<p class="disclaimer">{overview.disclaimer}</p>
			</article>

			<article class="models-card">
				<div class="section-head">
					<div>
						<p class="eyebrow">Performans ozeti</p>
						<h2>Model dogrulugu</h2>
					</div>
				</div>

				<div class="metric-list">
					<div>
						<span>Mevcut seri</span>
						<strong>{topSummary.accuracy.currentStreak} {topSummary.accuracy.streakDirection}</strong>
					</div>
					<div>
						<span>Yukselis isabeti</span>
						<strong>{Math.round(topSummary.accuracy.bullishAccuracy * 100)}%</strong>
					</div>
					<div>
						<span>Dusus isabeti</span>
						<strong>{Math.round(topSummary.accuracy.bearishAccuracy * 100)}%</strong>
					</div>
					<div>
						<span>Ornek sayisi</span>
						<strong>{topSummary.accuracy.lifetimeSampleSize}</strong>
					</div>
					<div>
						<span>Consensus ornek</span>
						<strong>{topSummary.accuracy.consensusSampleSize}</strong>
					</div>
					<div>
						<span>Ortak karar isabeti</span>
						<strong>{consensusAccuracyText(topSummary)}</strong>
					</div>
				</div>

				<div class="chip-list">
					{#each overview.activeModels as model}
						<span>{model}</span>
					{/each}
				</div>
			</article>
		</section>
	{/if}

	<section class="table-panel">
		<div class="section-head table-head">
			<div>
				<p class="eyebrow">Kripto</p>
				<h2>Binance spot / perpetual izleme</h2>
			</div>
		</div>

		<div class="asset-grid">
			{#each cryptoSummaries as summary}
				<a class="asset-row" href={`/markets/${summary.asset.market}/${summary.asset.symbol}`}>
					<div class="primary">
						<p class="symbol">{summary.asset.symbol}</p>
						<p class="name">{summary.asset.name} / {summary.asset.primaryExchangeCode}</p>
					</div>
					<div class="mini-chart-card">
						<svg viewBox="0 0 180 70" aria-label={`${summary.asset.symbol} mini candles`}>
							<rect class="chart-bg" x="0" y="0" width="180" height="70"></rect>
							{#each gridOffsets(4, 70) as y}
								<line class="grid-line" x1="0" x2="180" y1={y} y2={y}></line>
							{/each}
							{#each candleGeometry(summary.miniCandles, 180, 70) as candle}
								<line class:up={candle.rising} class:down={!candle.rising} class:live={candle.isLive} class="wick" x1={candle.x + candle.width / 2} x2={candle.x + candle.width / 2} y1={candle.highY} y2={candle.lowY}></line>
								<rect class:up={candle.rising} class:down={!candle.rising} class:live={candle.isLive} class="body" x={candle.x} y={candle.bodyY} width={candle.width} height={candle.bodyHeight}></rect>
							{/each}
						</svg>
					</div>
					<div><p class="label">Fiyat</p><p class="value">{formatter.format(summary.asset.lastPrice)}</p></div>
					<div><p class="label">24h</p><p class:up={summary.asset.changePercent24h >= 0} class:down={summary.asset.changePercent24h < 0} class="value">{formatPercent(summary.asset.changePercent24h)}</p></div>
					<div><p class="label">Ortak karar tahmini</p><p class="value">{consensusText(summary)}</p></div>
					<div><p class="label">Ortak islem</p><p class="value">{consensusTradeText(summary)}</p></div>
					<div><p class="label">Ortak islem isabeti</p><p class="value">{summary.accuracy.consensusTradeSampleSize > 0 ? `${Math.round(summary.accuracy.consensusTradeWinRate * 100)}%` : 'Bekleniyor'}</p></div>
					<div><p class="label">No-trade nedeni</p><p class="value">{summary.latestPrediction.tradeFilterReason || '-'}</p></div>
				</a>
			{/each}
		</div>
	</section>

	{#if lastError}
		<p class="error-banner">{lastError}</p>
	{/if}
</div>

<style>
	:global(body) {
		background:
			radial-gradient(circle at 20% 0%, rgba(74, 144, 226, 0.12), transparent 24%),
			radial-gradient(circle at 100% 0%, rgba(78, 205, 196, 0.09), transparent 26%),
			linear-gradient(180deg, #f8fbff 0%, #f2f5fb 45%, #edf2f8 100%);
	}
	.shell { max-width: 1320px; margin: 0 auto; padding: 36px 20px 64px; }
	.masthead, .hero-grid, .section-head, .asset-row, .stats-row { display: flex; gap: 20px; }
	.masthead, .section-head, .asset-row { justify-content: space-between; }
	.masthead { align-items: end; margin-bottom: 26px; flex-wrap: wrap; }
	h1, h2, p, strong { margin: 0; }
	h1 { max-width: 12ch; font-size: clamp(2.8rem, 6vw, 5.4rem); line-height: 0.94; letter-spacing: -0.04em; }
	h2 { font-size: clamp(1.4rem, 2vw, 2rem); color: #172233; }
	.eyebrow, .label, .name, .disclaimer { color: #748397; }
	.eyebrow { text-transform: uppercase; letter-spacing: 0.08em; }
	.intro, .analysis, .meta-card p { line-height: 1.6; color: #5f6e81; }
	.meta-card, .signal-card, .models-card, .table-panel, .asset-row {
		border: 1px solid rgba(173, 186, 204, 0.24);
		border-radius: 28px;
		background: rgba(255, 255, 255, 0.84);
		box-shadow: 0 20px 50px rgba(110, 133, 160, 0.12);
		backdrop-filter: blur(12px);
	}
	.meta-card, .signal-card, .models-card, .table-panel { padding: 24px; }
	.brand { flex: 1 1 720px; }
	.intro { max-width: 65ch; margin-top: 16px; }
	.meta-card { width: min(100%, 340px); }
	.meta-label, .badge { color: #2e7cf6; }
	.meta-card strong { display: block; margin: 4px 0 12px; font-size: 1.25rem; }
	.meta-row { display: inline-flex; align-items: center; gap: 10px; margin-top: 18px; font-size: 0.95rem; color: #698099; }
	.pulse { width: 10px; height: 10px; border-radius: 999px; background: #2e7cf6; box-shadow: 0 0 10px rgba(46, 124, 246, 0.26); }
	.live { opacity: 0.95; filter: drop-shadow(0 0 4px rgba(46, 124, 246, 0.12)); }
	.hero-grid { align-items: stretch; margin-bottom: 24px; flex-wrap: wrap; }
	.signal-card { flex: 1 1 700px; }
	.models-card { flex: 1 1 320px; }
	.badge, .chip-list span { border-radius: 999px; background: rgba(46, 124, 246, 0.08); padding: 9px 12px; }
	.metric-list, .asset-grid, .chip-list { display: grid; gap: 14px; }
	.metric-list { grid-template-columns: repeat(2, minmax(0, 1fr)); margin-top: 16px; }
	.metric-list div { border-radius: 18px; background: #f7f9fc; border: 1px solid #e7edf5; padding: 16px; }
	.stats-row { margin: 18px 0 16px; flex-wrap: wrap; }
	.stats-row div { min-width: 120px; }
	.stats-row span { display: block; margin-bottom: 8px; color: #748397; }
	.hero-chart, .mini-chart-card svg { width: 100%; height: auto; background: #fcfdff; border-radius: 18px; }
	.hero-chart { margin-top: 18px; }
	.chart-bg { fill: #fcfdff; }
	.grid-line { stroke: #dfe6f1; stroke-width: 1; }
	.grid-line.vertical { stroke: #edf2f7; }
	.wick { stroke-width: 1.35; }
	.body { stroke: none; }
	.up { color: #0fa67a; stroke: #0fa67a; fill: rgba(15, 166, 122, 0.9); }
	.down { color: #e55f61; stroke: #e55f61; fill: rgba(229, 95, 97, 0.9); }
	.asset-row { align-items: center; padding: 18px; transition: transform 160ms ease, border-color 160ms ease, background 160ms ease; }
	.asset-row:hover { transform: translateY(-2px); border-color: rgba(46, 124, 246, 0.18); background: rgba(255, 255, 255, 0.96); }
	.primary { min-width: 170px; }
	.mini-chart-card { width: 180px; min-width: 180px; }
	.symbol, .value { font-weight: 600; }
	.symbol { font-size: 1.05rem; }
	.table-panel { margin-top: 20px; }
	.table-head { align-items: center; margin-bottom: 14px; flex-wrap: wrap; }
	.error-banner { margin-top: 18px; padding: 12px 14px; border-radius: 14px; background: rgba(229, 95, 97, 0.12); color: #a33a3b; }
	@media (max-width: 980px) {
		.asset-row { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: start; }
		.mini-chart-card { width: 100%; min-width: 0; }
	}
	@media (max-width: 640px) {
		.asset-row, .metric-list { grid-template-columns: 1fr; }
		.section-head { flex-direction: column; align-items: flex-start; }
	}
</style>
