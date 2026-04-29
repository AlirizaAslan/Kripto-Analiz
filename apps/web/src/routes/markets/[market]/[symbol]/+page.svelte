<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import type { AssetDetail } from '$lib/types';
	import ProTradingChart from '$lib/chart/ProTradingChart.svelte';

	export let data: AssetDetail;

	let detail = data;
	let loading = false;
	let lastError = '';
	let refreshHandle: ReturnType<typeof setInterval> | undefined;

	function formatPrice(value: number) {
		return new Intl.NumberFormat('tr-TR', {
			style: 'currency',
			currency: 'USD',
			maximumFractionDigits: 2
		}).format(value);
	}

	function formatPercent(value: number) {
		return `${value > 0 ? '+' : ''}${value.toFixed(2)}%`;
	}

	function formatRate(value: number) {
		if (!Number.isFinite(value)) return 'Bekleniyor';
		return `${Math.round(value * 100)}%`;
	}

	function tradeRateText(value: number, sampleSize: number) {
		if (sampleSize <= 0) return 'Bekleniyor';
		return formatRate(value);
	}

	function historyActionLabel(item: AssetDetail['predictionHistory'][number]) {
		return item.tradeAllowed ? tradeActionLabel(item.tradeAction) : 'TAHMIN';
	}

	function tradeActionLabel(value: AssetDetail['prediction']['tradeAction']) {
		if (value === 'buy') return 'AL';
		if (value === 'sell') return 'SAT';
		if (value === 'hold') return 'BEKLE';
		return 'ISLEM YOK';
	}

	function directionLabel(value: 'up' | 'down' | 'neutral' | '') {
		if (value === 'up') return 'Yukselecek';
		if (value === 'down') return 'Dusecek';
		if (value === '') return 'Bekleniyor';
		return 'Yatay';
	}

	function tradeResultDirection(item: AssetDetail['predictionHistory'][number]) {
		if (item.isPending) return '';
		if (item.realizedDirection !== 'neutral' || !item.tradeAllowed) return item.realizedDirection;
		if (item.tradeAction === 'buy') return item.wasCorrect ? 'up' : 'down';
		if (item.tradeAction === 'sell') return item.wasCorrect ? 'down' : 'up';
		return item.realizedDirection;
	}

	function qualityLabel(value: string) {
		if (value === 'full_depth') return 'Canli derinlik';
		if (value === 'top_of_book_only') return 'Sinirli derinlik';
		return 'Isinma / degrade';
	}

	function streakLabel(value: string) {
		return value === 'win' ? 'dogru seri' : 'yanlis seri';
	}

	function consensusDirection() {
		if (!detail.prediction.consensusActive) return null;
		return detail.prediction.consensusDirection || null;
	}

	function consensusAccuracyText() {
		if (detail.prediction.consensusActive) {
			return directionLabel(detail.prediction.consensusDirection);
		}
		return 'Bekleniyor';
	}

	function consensusTradeLabel() {
		if (detail.prediction.tradeAction === 'buy') return 'AL';
		if (detail.prediction.tradeAction === 'sell') return 'SAT';
		return 'ISLEM YOK';
	}

	function consensusTradeNote() {
		return detail.prediction.tradeFilterReason || detail.prediction.consensusSummary || 'Ortak islem sinyali bekleniyor.';
	}

	function consensusWinRateText() {
		if (detail.accuracy.consensusSampleSize > 0) {
			return formatRate(detail.accuracy.consensusWinRate);
		}
		return 'Bekleniyor';
	}

	$: orderedHistory = [...detail.predictionHistory].sort(
		(left, right) => new Date(right.targetCandleStart).getTime() - new Date(left.targetCandleStart).getTime()
	);
	$: executableHistory = orderedHistory.filter(
		(item) => item.tradeAllowed && item.predictedDirection !== 'neutral'
	);
	$: nonNeutralHistory = orderedHistory.filter((item) => item.predictedDirection !== 'neutral');
	$: displayedHistory = executableHistory.length ? executableHistory : nonNeutralHistory;
	$: historyLabel = executableHistory.length ? 'Son acilan islemler' : 'Son yonlu tahminler';
	$: resolvedHistory = detail.predictionHistory.filter((item) => !item.isPending);
	$: accuracySampleSize = detail.accuracy.sampleSize || resolvedHistory.length;
	$: hasAccuracySample = accuracySampleSize > 0;
	$: hasCurrentHistoryItem = orderedHistory.some(
		(item) => item.targetCandleStart === detail.prediction.targetCandleStart
	);
	$: liveTradePreview =
		detail.prediction.tradeAllowed && !hasCurrentHistoryItem
			? {
					targetCandleStart: detail.prediction.targetCandleStart,
					predictedDirection: detail.prediction.predictedDirection,
					realizedDirection: '' as 'up' | 'down' | 'neutral' | '',
					confidenceScore: detail.prediction.confidenceScore,
					wasCorrect: false,
					tradeAllowed: true,
					tradeAction: detail.prediction.tradeAction,
					isPending: true
				}
			: null;

	async function refresh() {
		loading = true;
		try {
			const response = await fetch(
				`/api/markets/${detail.asset.market}/symbols/${detail.asset.symbol}?t=${Date.now()}`,
				{ cache: 'no-store' }
			);
			if (!response.ok) throw new Error('Sembol yenilenemedi');
			detail = (await response.json()) as AssetDetail;
			lastError = '';
		} catch (error) {
			lastError = error instanceof Error ? error.message : 'Sembol yenilenemedi';
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
	<title>{detail.asset.symbol} | PulseAlpha 1m tahmin</title>
</svelte:head>

<div class="detail-shell">
	<a class="back" href="/">Piyasa ozetine don</a>

	<section class="headline">
		<div>
			<p class="tag">{detail.asset.market} / {detail.asset.primaryExchangeCode} / sonraki {detail.asset.nextCandleInterval}</p>
			<h1>{detail.asset.symbol}</h1>
			<p class="summary">{detail.asset.thesis}</p>
		</div>

		<div class="headline-stats">
			<div><span>Son fiyat</span><strong>{formatPrice(detail.asset.lastPrice)}</strong></div>
			<div><span>24h</span><strong class:up={detail.asset.changePercent24h >= 0} class:down={detail.asset.changePercent24h < 0}>{formatPercent(detail.asset.changePercent24h)}</strong></div>
			<div><span>Sinyal kalitesi</span><strong>{qualityLabel(detail.prediction.signalQuality)}</strong></div>
			<div><span>Durum</span><strong>{loading ? 'Yenileniyor' : 'Canli'}</strong></div>
		</div>
	</section>

	<section class="top-grid">
		<article class="chart-panel">
			<ProTradingChart {detail} />

			<div class="range-row">
				<div><span>24s dusuk</span><strong>{formatPrice(detail.asset.range24h.low)}</strong></div>
				<div><span>24s yuksek</span><strong>{formatPrice(detail.asset.range24h.high)}</strong></div>
				<div><span>Hedef mum</span><strong>{new Date(detail.prediction.targetCandleStart).toLocaleTimeString('tr-TR')}</strong></div>
				<div><span>Canli mum</span><strong>{detail.candles[detail.candles.length - 1]?.isLive ? 'Gorunur' : 'Sadece kapananlar'}</strong></div>
			</div>
			<p class="view-note">Timeframe secimi, mevcut snapshot icindeki gercek veya fallback candle setlerini gosterir.</p>

			{#if consensusDirection()}
				<div class="consensus-badge" class:up={consensusDirection() === 'up'} class:down={consensusDirection() === 'down'} class:neutral={consensusDirection() === 'neutral'}>
					<span>Ortak karar</span>
					<strong>{directionLabel(consensusDirection()!)}</strong>
				</div>
			{/if}

			<div class="trade-callout" class:up={detail.prediction.tradeAction === 'buy'} class:down={detail.prediction.tradeAction === 'sell'} class:neutral={detail.prediction.predictedDirection === 'neutral'} class:inactive={!detail.prediction.tradeAllowed}>
				<div>
					<span>Ortak islem</span>
					<strong>{consensusTradeLabel()}</strong>
				</div>
				<p>{consensusTradeNote()}</p>
			</div>
		</article>

		<article class="panel">
			<div class="panel-head">
				<div>
					<p class="eyebrow">Ortak model karari</p>
					<h2>Islem durumu</h2>
				</div>
				<p class="risk">{detail.prediction.riskLabel}</p>
			</div>

			<div class="stat-grid">
				<div><span>Ortak islem</span><strong>{consensusTradeLabel()}</strong></div>
				<div><span>Ortak karar</span><strong>{consensusAccuracyText()}</strong></div>
				<div><span>Ortak islem isabeti</span><strong>{detail.accuracy.consensusTradeSampleSize > 0 ? formatRate(detail.accuracy.consensusTradeWinRate) : 'Bekleniyor'}</strong></div>
				<div><span>Consensus ornek</span><strong>{detail.accuracy.consensusTradeSampleSize || detail.accuracy.consensusSampleSize}</strong></div>
				<div><span>Rejim</span><strong>{detail.prediction.regimeLabel}</strong></div>
			</div>

			<p class="narrative">{consensusTradeNote()}</p>
			<p class="disclaimer">{detail.disclaimer}</p>
		</article>
	</section>

	<section class="bottom-grid">
		<article class="panel">
			<div class="panel-head">
				<div>
					<p class="eyebrow">Model dogrulugu</p>
					<h2>Ayrintili performans paneli</h2>
				</div>
			</div>

			<div class="accuracy-grid">
				<div><span>Genel isabet</span><strong>{tradeRateText(detail.accuracy.winRate, accuracySampleSize)}</strong></div>
				<div><span>Son 5 isabet</span><strong>{hasAccuracySample ? formatRate(detail.accuracy.recentWindowWinRate) : 'Bekleniyor'}</strong></div>
				<div><span>Son 6 isabet</span><strong>{hasAccuracySample ? formatRate(detail.accuracy.recent6WinRate) : 'Bekleniyor'}</strong></div>
				<div><span>Son 7 isabet</span><strong>{hasAccuracySample ? formatRate(detail.accuracy.recent7WinRate) : 'Bekleniyor'}</strong></div>
				<div><span>Islem isabeti</span><strong>{tradeRateText(detail.accuracy.tradeWinRate, detail.accuracy.tradeSampleSize)}</strong></div>
				<div><span>Mevcut seri</span><strong>{hasAccuracySample ? `${detail.accuracy.currentStreak} ${streakLabel(detail.accuracy.streakDirection)}` : 'Bekleniyor'}</strong></div>
				<div><span>1m isabet</span><strong>{hasAccuracySample && detail.accuracy.horizonSampleSizes?.['1m'] ? formatRate(detail.accuracy.horizonWinRates['1m']) : 'Bekleniyor'}</strong></div>
				<div><span>5m isabet</span><strong>{hasAccuracySample && detail.accuracy.horizonSampleSizes?.['5m'] ? formatRate(detail.accuracy.horizonWinRates['5m']) : 'Bekleniyor'}</strong></div>
				<div><span>15m isabet</span><strong>{hasAccuracySample && detail.accuracy.horizonSampleSizes?.['15m'] ? formatRate(detail.accuracy.horizonWinRates['15m']) : 'Bekleniyor'}</strong></div>
				<div><span>1h isabet</span><strong>{hasAccuracySample && detail.accuracy.horizonSampleSizes?.['1h'] ? formatRate(detail.accuracy.horizonWinRates['1h']) : 'Bekleniyor'}</strong></div>
				<div><span>Ornek sayisi</span><strong>{accuracySampleSize}</strong></div>
				<div><span>Islem ornek</span><strong>{detail.accuracy.tradeSampleSize}</strong></div>
			</div>

			<div class="history-list scroll-list">
				<p class="column-label">{historyLabel}</p>
				{#if liveTradePreview || displayedHistory.length}
					{#if liveTradePreview}
						<div class="history-row pending-row">
							<span>{new Date(liveTradePreview.targetCandleStart).toLocaleTimeString('tr-TR')}</span>
							<span>{tradeActionLabel(liveTradePreview.tradeAction)} / {directionLabel(liveTradePreview.predictedDirection)} -> Bekleniyor</span>
							<span class="neutral">Acik islem</span>
						</div>
					{/if}
					{#each displayedHistory as item}
						<div class="history-row" class:pending-row={item.isPending}>
							<span>{new Date(item.targetCandleStart).toLocaleTimeString('tr-TR')}</span>
							<span>{historyActionLabel(item)} / {directionLabel(item.predictedDirection)} -> {directionLabel(tradeResultDirection(item))}</span>
							<span class:good={item.wasCorrect && !item.isPending} class:bad={!item.wasCorrect && !item.isPending} class:neutral={item.isPending}>{item.isPending ? 'Acik islem' : item.wasCorrect ? 'Dogru' : 'Kacirdi'}</span>
						</div>
					{/each}
				{:else}
					<div class="history-empty">Henuz acik veya kapanmis islem yok.</div>
				{/if}
			</div>
		</article>

		<article class="panel side-panel">
			<div class="panel-head">
				<div>
					<p class="eyebrow">Derinlik + suruculer</p>
					<h2>Sinyal neden olustu</h2>
				</div>
			</div>

			<div class="depth-meta">
				<div><span>En iyi alim</span><strong>{formatPrice(detail.depth.bestBid)}</strong></div>
				<div><span>En iyi satis</span><strong>{formatPrice(detail.depth.bestAsk)}</strong></div>
				<div><span>Spread bps</span><strong>{detail.depth.spreadBps}</strong></div>
				<div><span>Mikro fiyat</span><strong>{formatPrice(detail.depth.microPrice)}</strong></div>
			</div>

			<div class="feature-list">
				{#each detail.prediction.topFeatures as feature}
					<div class="feature-card">
						<div class="feature-head">
							<strong>{feature.name}</strong>
							<span>{feature.contribution > 0 ? '+' : ''}{feature.contribution}</span>
						</div>
						<p>{feature.summary}</p>
					</div>
				{/each}
			</div>

			<div class="watchlist-note"><p>{detail.watchlistNote}</p></div>
		</article>
	</section>

	{#if lastError}
		<p class="error-banner">{lastError}</p>
	{/if}
</div>

<style>
	.detail-shell { max-width: 1320px; margin: 0 auto; padding: 28px 20px 60px; }
	.back, .tag, .eyebrow, .summary, .disclaimer, .column-label, .panel-head p, .feature-card p, .history-row span:first-child { color: #748397; }
	h1, h2, p, strong, span { margin: 0; }
	h1 { font-size: clamp(2.6rem, 5vw, 4.4rem); line-height: 0.95; margin: 8px 0 12px; }
	h2 { font-size: clamp(1.3rem, 2vw, 1.9rem); color: #162131; }
	.headline, .top-grid, .bottom-grid, .panel-head, .headline-stats, .range-row, .depth-meta, .feature-head, .history-row { display: flex; gap: 18px; }
	.headline, .panel-head, .feature-head, .history-row { justify-content: space-between; }
	.headline, .panel {
		border: 1px solid rgba(173, 186, 204, 0.26);
		border-radius: 28px;
		background: rgba(255, 255, 255, 0.84);
		box-shadow: 0 18px 42px rgba(109, 134, 163, 0.12);
		backdrop-filter: blur(10px);
	}
	.headline { align-items: end; padding: 26px; margin: 14px 0 20px; flex-wrap: wrap; }
	.headline-stats, .range-row, .depth-meta { flex-wrap: wrap; }
	.headline-stats div, .range-row div, .depth-meta div { min-width: 120px; }
	.headline-stats span, .range-row span, .depth-meta span { display: block; margin-bottom: 8px; }
	.top-grid, .bottom-grid { align-items: stretch; flex-wrap: wrap; margin-bottom: 18px; }
	.chart-panel { flex: 1 1 760px; min-width: 0; }
	.top-grid > .panel:not(.chart-panel) { flex: 1 1 360px; }
	.bottom-grid > .panel:first-child { flex: 1 1 700px; }
	.side-panel { flex: 1 1 320px; }
	.panel { padding: 22px; }
	.up, .good { color: #0fa67a; stroke: #0fa67a; fill: rgba(15, 166, 122, 0.9); }
	.down, .bad, .risk { color: #e55f61; stroke: #e55f61; fill: rgba(229, 95, 97, 0.9); }
	.neutral { color: #76879a; stroke: #76879a; fill: rgba(118, 135, 154, 0.85); }
	.stat-grid, .accuracy-grid, .feature-list { display: grid; gap: 14px; }
	.stat-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
	.accuracy-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); margin-bottom: 16px; }
	.stat-grid div, .accuracy-grid div, .feature-card, .watchlist-note { padding: 16px; border-radius: 18px; background: #f7f9fc; border: 1px solid #e7edf5; }
	.feature-list, .history-list { margin-top: 16px; }
	.feature-card, .history-row { background: #f7f9fc; border: 1px solid #e7edf5; border-radius: 16px; padding: 14px; }
	.history-list { display: grid; gap: 10px; }
	.history-empty {
		padding: 14px;
		border-radius: 16px;
		background: #f7f9fc;
		border: 1px solid #e7edf5;
		color: #748397;
		line-height: 1.6;
	}
	.scroll-list {
		max-height: 320px;
		overflow-y: auto;
		padding-right: 6px;
	}
	.scroll-list::-webkit-scrollbar {
		width: 10px;
	}
	.scroll-list::-webkit-scrollbar-thumb {
		background: #d4deea;
		border-radius: 999px;
	}
	.history-row { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
	.pending-row {
		border-style: dashed;
		background: #f5f9ff;
	}
	.consensus-badge {
		display: inline-flex;
		align-items: center;
		gap: 12px;
		margin-top: 16px;
		padding: 12px 16px;
		border-radius: 999px;
		border: 1px solid #dce6f0;
		background: #f8fbff;
	}
	.consensus-badge span {
		color: #748397;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 0.72rem;
	}
	.consensus-badge strong {
		font-size: 0.96rem;
	}
	.trade-callout {
		display: grid;
		gap: 8px;
		margin-top: 16px;
		padding: 16px 18px;
		border-radius: 20px;
		border: 1px solid #dce6f0;
		background: linear-gradient(180deg, #fbfdff 0%, #f5f9ff 100%);
	}
	.trade-callout div {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}
	.trade-callout span {
		color: #748397;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 0.72rem;
	}
	.trade-callout strong {
		font-size: 1.15rem;
		color: #162131;
	}
	.trade-callout p {
		color: #5e6c80;
		line-height: 1.5;
	}
	.trade-callout.up {
		box-shadow: 0 0 0 1px rgba(15, 166, 122, 0.08) inset;
	}
	.trade-callout.down {
		box-shadow: 0 0 0 1px rgba(229, 95, 97, 0.08) inset;
	}
	.trade-callout.neutral {
		box-shadow: 0 0 0 1px rgba(118, 135, 154, 0.08) inset;
	}
	.trade-callout.inactive {
		background: linear-gradient(180deg, #fbfcfe 0%, #f7f9fc 100%);
	}
	.consensus-badge.up {
		box-shadow: 0 0 0 1px rgba(15, 166, 122, 0.08) inset;
	}
	.consensus-badge.down {
		box-shadow: 0 0 0 1px rgba(229, 95, 97, 0.08) inset;
	}
	.consensus-badge.neutral {
		box-shadow: 0 0 0 1px rgba(118, 135, 154, 0.08) inset;
	}
	.narrative, .summary, .disclaimer, .watchlist-note p, .view-note { line-height: 1.6; color: #5e6c80; }
	.error-banner { margin-top: 18px; padding: 12px 14px; border-radius: 14px; background: rgba(229, 95, 97, 0.12); color: #a33a3b; }
	@media (max-width: 760px) {
		.stat-grid, .accuracy-grid, .history-row { grid-template-columns: 1fr; }
		.panel-head, .feature-head { flex-direction: column; align-items: flex-start; }
	}
</style>
