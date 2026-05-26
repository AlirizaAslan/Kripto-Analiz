<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import type { AssetDetail, SymbolStatistics } from '$lib/types';
	import ProTradingChart from '$lib/chart/ProTradingChart.svelte';
	import SymbolStatisticsPanel from '$lib/SymbolStatisticsPanel.svelte';

	export let data: { detail: AssetDetail; statistics: SymbolStatistics | null };

	type HistoryItem = AssetDetail['predictionHistory'][number];

	let detail = data.detail;
	let statistics = data.statistics;
	let loading = false;
	let lastError = '';
	let refreshHandle: ReturnType<typeof setInterval> | undefined;
	let historyPanelTab: 'history' | 'models' = 'history';
	let currentModelDecision: HistoryItem;

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
		if (item.predictedDirection === 'up') return 'AL';
		if (item.predictedDirection === 'down') return 'SAT';
		return tradeActionLabel(item.tradeAction);
	}

	function shouldShowInHistoryPreview() {
		return (
			detail.prediction.consensusActive &&
			(detail.prediction.consensusDirection === 'up' || detail.prediction.consensusDirection === 'down') &&
			detail.prediction.predictedDirection === detail.prediction.consensusDirection &&
			(detail.prediction.tradeAction === 'buy' || detail.prediction.tradeAction === 'sell')
		);
	}

	function outcomeLabel(item: HistoryItem) {
		if (item.isPending) return 'Bekleniyor';
		return item.wasCorrect ? 'Dogru' : 'Yanlis';
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
		if (!detail.prediction.tradeAllowed) return 'ISLEM YOK';
		if (detail.prediction.tradeAction === 'buy') return 'AL';
		if (detail.prediction.tradeAction === 'sell') return 'SAT';
		return 'ISLEM YOK';
	}

	function consensusTradeNote() {
		return detail.prediction.tradeFilterReason || detail.prediction.consensusSummary || 'Ortak islem sinyali bekleniyor.';
	}

	function predictionHeadlineLabel() {
		if (detail.prediction.tradeAction === 'buy') return 'AL';
		if (detail.prediction.tradeAction === 'sell') return 'SAT';
		if (detail.prediction.predictedDirection === 'up') return 'AL ADAYI';
		if (detail.prediction.predictedDirection === 'down') return 'SAT ADAYI';
		return 'BEKLE';
	}

	function predictionHeadlineClass() {
		if (detail.prediction.tradeAction === 'buy') return 'up';
		if (detail.prediction.tradeAction === 'sell') return 'down';
		if (detail.prediction.predictedDirection === 'up') return 'up';
		if (detail.prediction.predictedDirection === 'down') return 'down';
		return 'neutral';
	}

	function modelActionLabel(direction: AssetDetail['prediction']['predictedDirection']) {
		if (direction === 'up') return 'AL';
		if (direction === 'down') return 'SAT';
		return 'BEKLE';
	}

	function modelActionClass(direction: AssetDetail['prediction']['predictedDirection']) {
		if (direction === 'up') return 'up';
		if (direction === 'down') return 'down';
		return 'neutral';
	}

	function modelDecisionSummary() {
		return detail.prediction.modelComponents
			.map((component) => `${component.name}: ${modelActionLabel(component.predictedDirection)}`)
			.join(' | ');
	}

	function consensusWinRateText() {
		if (detail.accuracy.consensusSampleSize > 0) {
			return formatRate(detail.accuracy.consensusWinRate);
		}
		return 'Bekleniyor';
	}

	function predictionModelDirections(): HistoryItem['modelDirections'] {
		return Object.fromEntries(
			detail.prediction.modelComponents.map((component) => [
				component.name,
				component.predictedDirection
			])
		);
	}

	function modelDirectionEntries(item: HistoryItem) {
		const modelOrder = detail.prediction.modelComponents.map((component) => component.name);
		const seen = new Set<string>();
		const entries: { name: string; direction: HistoryItem['predictedDirection'] | '' }[] = [];
		const directions = item.modelDirections ?? {};

		for (const name of modelOrder) {
			seen.add(name);
			entries.push({ name, direction: directions[name] ?? '' });
		}

		for (const [name, direction] of Object.entries(directions)) {
			if (!seen.has(name)) entries.push({ name, direction });
		}

		return entries;
	}

	function consensusStatusText(item: HistoryItem) {
		if (item.consensusActive) return `Ortak karar: ${directionLabel(item.consensusDirection)}`;
		return 'Ortak karar yok';
	}

	function consensusReasonText(item: HistoryItem) {
		return item.tradeFilterReason || 'Modeller ayni yonde yeterli cogunluk olusturmadi.';
	}

	function modelOutcomeText(item: HistoryItem, direction: HistoryItem['predictedDirection'] | '') {
		if (item.isPending || !item.realizedDirection || !direction) return 'Bekleniyor';
		return direction === item.realizedDirection ? 'Dogru' : 'Yanlis';
	}

	$: orderedHistory = [...detail.predictionHistory].sort(
		(left, right) => new Date(right.targetCandleStart).getTime() - new Date(left.targetCandleStart).getTime()
	);
	$: displayedHistory = orderedHistory;
	$: historyLabel = 'Son hedef mum tahminleri';
	$: resolvedHistory = detail.predictionHistory.filter((item) => !item.isPending);
	$: accuracySampleSize = detail.accuracy.sampleSize || resolvedHistory.length;
	$: hasAccuracySample = accuracySampleSize > 0;
	$: hasCurrentHistoryItem = orderedHistory.some(
		(item) => item.targetCandleStart === detail.prediction.targetCandleStart
	);
	$: liveTradePreview =
		!hasCurrentHistoryItem && shouldShowInHistoryPreview()
			? {
					targetCandleStart: detail.prediction.targetCandleStart,
					predictedDirection: detail.prediction.predictedDirection,
					realizedDirection: '' as 'up' | 'down' | 'neutral' | '',
					confidenceScore: detail.prediction.confidenceScore,
					wasCorrect: false,
					tradeAllowed: detail.prediction.tradeAllowed,
					tradeAction: detail.prediction.tradeAction,
					modelDirections: predictionModelDirections(),
					consensusActive: detail.prediction.consensusActive,
					consensusDirection: detail.prediction.consensusDirection,
					consensusStrength: detail.prediction.consensusStrength,
					tradeFilterReason: detail.prediction.tradeFilterReason,
					isPending: true
				}
			: null;
	$: currentModelDecision = {
		targetCandleStart: detail.prediction.targetCandleStart,
		predictedDirection: detail.prediction.predictedDirection,
		realizedDirection: '',
		confidenceScore: detail.prediction.confidenceScore,
		wasCorrect: false,
		tradeAllowed: detail.prediction.tradeAllowed,
		tradeAction: detail.prediction.tradeAction,
		modelDirections: predictionModelDirections(),
		consensusActive: detail.prediction.consensusActive,
		consensusDirection: detail.prediction.consensusDirection,
		consensusStrength: detail.prediction.consensusStrength,
		tradeFilterReason: detail.prediction.tradeFilterReason,
		isPending: true
	};
	$: modelDecisionHistory = displayedHistory.filter(
		(item) => item.targetCandleStart !== currentModelDecision.targetCandleStart
	);

	async function refresh() {
		loading = true;
		try {
			const [detailResponse, statsResponse] = await Promise.all([
				fetch(`/api/markets/${detail.asset.market}/symbols/${detail.asset.symbol}?t=${Date.now()}`, { cache: 'no-store' }),
				fetch(`/api/markets/${detail.asset.market}/symbols/${detail.asset.symbol}/statistics?t=${Date.now()}`, { cache: 'no-store' })
			]);
			if (!detailResponse.ok) throw new Error('Sembol yenilenemedi');
			detail = (await detailResponse.json()) as AssetDetail;
			if (statsResponse.ok) {
				statistics = (await statsResponse.json()) as SymbolStatistics;
			}
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
	<div class="page-links">
		<a class="back" href="/">Piyasa ozetine don</a>
		<a class="stats-link" href={`/statistics/${detail.asset.market}/${detail.asset.symbol}`}>
			Ayrintili istatistikleri ac
		</a>
	</div>

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

			<div class={`prediction-banner ${predictionHeadlineClass()}`}>
				<div>
					<span>Sonraki 1 dakikalik tahmin</span>
					<strong>{predictionHeadlineLabel()}</strong>
				</div>
				<p>{consensusTradeNote()}</p>
			</div>

			<div class="trade-callout" class:up={detail.prediction.tradeAction === 'buy'} class:down={detail.prediction.tradeAction === 'sell'} class:neutral={detail.prediction.predictedDirection === 'neutral'} class:inactive={!detail.prediction.tradeAllowed}>
				<div>
					<span>Ortak islem</span>
					<strong>{consensusTradeLabel()}</strong>
				</div>
				<p>{consensusTradeNote()}</p>
			</div>

			<details class="prediction-details">
				<summary>Daha detayli tahmini goster</summary>
				<div class="prediction-detail-grid">
					<div><span>Final yon</span><strong>{directionLabel(detail.prediction.predictedDirection)}</strong></div>
					<div><span>Guven skoru</span><strong>{Math.round(detail.prediction.confidenceScore * 100)}%</strong></div>
					<div><span>Yukselis olasiligi</span><strong>{Math.round(detail.prediction.upProbability * 100)}%</strong></div>
					<div><span>Dusus olasiligi</span><strong>{Math.round(detail.prediction.downProbability * 100)}%</strong></div>
					<div><span>Yatay olasilik</span><strong>{Math.round(detail.prediction.neutralProbability * 100)}%</strong></div>
					<div><span>Risk etiketi</span><strong>{detail.prediction.riskLabel}</strong></div>
				</div>
				<p class="prediction-explanation">{detail.prediction.explanation}</p>
			</details>
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
			<div class="model-signal-grid">
				{#each detail.prediction.modelComponents as component}
					<div class="model-signal-card">
						<div class="feature-head">
							<strong>{component.name}</strong>
							<span class={`model-badge ${modelActionClass(component.predictedDirection)}`}>
								{modelActionLabel(component.predictedDirection)}
							</span>
						</div>
						<p>{component.summary}</p>
						<div class="model-meta-row">
							<span>Olasilik {Math.round(component.probability * 100)}%</span>
							<span>Gecmis {formatRate(component.historicalHitRate)}</span>
							<span>Son seri {formatRate(component.recentWinRate)}</span>
						</div>
					</div>
				{/each}
			</div>
			<p class="disclaimer">{detail.disclaimer}</p>
		</article>
	</section>

	<section class="bottom-grid">
		<article class="panel">
			<div class="panel-head">
				<div>
					<p class="eyebrow">Model dogrulugu</p>
					<h2>Istatistik ozeti</h2>
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

			<details class="history-accordion">
				<summary>
					{#if displayedHistory.length > 0}
						{@const latest = displayedHistory[0]}
						<div class="summary-content">
							<span class="summary-label">Son Tahmin:</span>
							<strong>{new Date(latest.targetCandleStart).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })}</strong>
							<span class="summary-divider">|</span>
							<strong class={latest.predictedDirection === 'up' ? 'up' : latest.predictedDirection === 'down' ? 'down' : 'neutral'}>
								{latest.predictedDirection === 'up' ? 'AL' : latest.predictedDirection === 'down' ? 'SAT' : '-'}
							</strong>
							<span class="summary-divider">|</span>
							<span>
								{#if latest.isPending}
									<span class="neutral">Bekliyor</span>
								{:else if latest.wasCorrect}
									<span class="badge badge-success">Başarılı</span>
								{:else}
									<span class="badge badge-error">Hatalı</span>
								{/if}
							</span>
						</div>
					{:else}
						<div class="summary-content">
							<span class="summary-label">Geçmiş tahmin bulunmuyor</span>
						</div>
					{/if}
				</summary>
				<div class="accordion-body">
					<table class="history-table">
						<thead>
							<tr>
								<th>Saat</th>
								<th>Tahmin</th>
								<th>Gerçekleşen</th>
								<th>Sonuç</th>
							</tr>
						</thead>
						<tbody>
							{#each displayedHistory.slice(1, 10) as item}
								<tr>
									<td>{new Date(item.targetCandleStart).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' })}</td>
									<td><strong class={item.predictedDirection === 'up' ? 'up' : item.predictedDirection === 'down' ? 'down' : 'neutral'}>{item.predictedDirection === 'up' ? 'AL' : item.predictedDirection === 'down' ? 'SAT' : '-'}</strong></td>
									<td>
										{#if item.isPending}
											<span class="neutral">Bekliyor</span>
										{:else}
											<strong class={item.realizedDirection === 'up' ? 'up' : item.realizedDirection === 'down' ? 'down' : 'neutral'}>{item.realizedDirection === 'up' ? 'Yükseliş' : item.realizedDirection === 'down' ? 'Düşüş' : 'Yatay'}</strong>
										{/if}
									</td>
									<td>
										{#if item.isPending}
											-
										{:else if item.wasCorrect}
											<span class="badge badge-success">Başarılı</span>
										{:else}
											<span class="badge badge-error">Hatalı</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</details>

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

			<div class="analysis-callout">
				<span>Model karar ozeti</span>
				<strong>{modelDecisionSummary()}</strong>
				<p>Her modelin yonu yukarida AL veya SAT olarak gosterilir; bu blok toplu model analizini tek satirda verir.</p>
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

	<SymbolStatisticsPanel item={statistics} {detail} />

	{#if lastError}
		<p class="error-banner">{lastError}</p>
	{/if}
</div>

<style>
	.detail-shell { max-width: 1320px; margin: 0 auto; padding: 28px 20px 60px; }
	.page-links { display: flex; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
	.back, .tag, .eyebrow, .summary, .disclaimer, .panel-head p, .feature-card p { color: #748397; }
	.stats-link, .stats-cta {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 10px 14px;
		border-radius: 12px;
		background: rgba(46, 124, 246, 0.08);
		color: #1c4da1;
		font-weight: 700;
	}
	h1, h2, p, strong, span { margin: 0; }
	h1 { font-size: clamp(2.6rem, 5vw, 4.4rem); line-height: 0.95; margin: 8px 0 12px; }
	h2 { font-size: clamp(1.3rem, 2vw, 1.9rem); color: #162131; }
	.headline, .top-grid, .bottom-grid, .panel-head, .headline-stats, .range-row, .depth-meta, .feature-head { display: flex; gap: 18px; }
	.headline, .panel-head, .feature-head { justify-content: space-between; }
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
	.up { color: #0fa67a; stroke: #0fa67a; fill: rgba(15, 166, 122, 0.9); }
	.down, .risk { color: #e55f61; stroke: #e55f61; fill: rgba(229, 95, 97, 0.9); }
	.neutral { color: #76879a; stroke: #76879a; fill: rgba(118, 135, 154, 0.85); }
	.stat-grid, .accuracy-grid, .feature-list { display: grid; gap: 14px; }
	.stat-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
	.accuracy-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); margin-bottom: 16px; }
	.stat-grid div, .accuracy-grid div, .feature-card, .watchlist-note, .model-signal-card, .analysis-callout { padding: 16px; border-radius: 18px; background: #f7f9fc; border: 1px solid #e7edf5; }
	.feature-list { margin-top: 16px; }
	.feature-card { background: #f7f9fc; border: 1px solid #e7edf5; border-radius: 16px; padding: 14px; }
	.model-signal-grid {
		display: grid;
		gap: 12px;
		margin-top: 16px;
	}
	.model-meta-row {
		display: flex;
		flex-wrap: wrap;
		gap: 10px;
		margin-top: 10px;
		color: #5e6c80;
		font-size: 0.92rem;
	}
	.model-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 6px 10px;
		border-radius: 999px;
		font-size: 0.8rem;
		font-weight: 700;
	}
	.model-badge.up {
		background: rgba(15, 166, 122, 0.12);
		color: #0fa67a;
	}
	.model-badge.down {
		background: rgba(229, 95, 97, 0.12);
		color: #e55f61;
	}
	.model-badge.neutral {
		background: rgba(118, 135, 154, 0.12);
		color: #76879a;
	}
	.analysis-callout {
		display: grid;
		gap: 8px;
		margin-top: 16px;
		background: linear-gradient(180deg, #fbfdff 0%, #f5f9ff 100%);
	}
	.analysis-callout span {
		color: #748397;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 0.72rem;
	}
	.analysis-callout p {
		color: #5e6c80;
		line-height: 1.5;
	}
	.stats-callout {
		display: grid;
		gap: 12px;
		margin-top: 16px;
		padding: 18px;
		border-radius: 20px;
		background: linear-gradient(180deg, #fbfdff 0%, #f5f9ff 100%);
		border: 1px solid #dce6f0;
	}
	.stats-callout span {
		display: block;
		margin-bottom: 8px;
		color: #748397;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 0.72rem;
	}
	.stats-callout p {
		color: #5e6c80;
		line-height: 1.6;
	}
	.stats-trade-pill {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 14px 16px;
		border-radius: 16px;
		background: #ffffff;
		border: 1px solid #dce6f0;
	}
	.stats-trade-pill span {
		display: block;
		margin-bottom: 0;
		color: #748397;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 0.72rem;
	}
	.stats-trade-pill strong {
		font-size: 1rem;
	}
	.stats-trade-pill.up {
		box-shadow: 0 0 0 1px rgba(15, 166, 122, 0.08) inset;
	}
	.stats-trade-pill.down {
		box-shadow: 0 0 0 1px rgba(229, 95, 97, 0.08) inset;
	}
	.stats-trade-pill.neutral {
		box-shadow: 0 0 0 1px rgba(118, 135, 154, 0.08) inset;
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
	.prediction-banner {
		display: grid;
		gap: 8px;
		margin-top: 16px;
		padding: 18px 20px;
		border-radius: 22px;
		border: 1px solid #dce6f0;
		background: linear-gradient(180deg, #fbfdff 0%, #f5f9ff 100%);
	}
	.prediction-banner div {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}
	.prediction-banner span {
		color: #748397;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		font-size: 0.72rem;
	}
	.prediction-banner strong {
		font-size: 1.4rem;
		color: #162131;
	}
	.prediction-banner p {
		color: #5e6c80;
		line-height: 1.5;
	}
	.prediction-banner.up {
		box-shadow: 0 0 0 1px rgba(15, 166, 122, 0.08) inset;
	}
	.prediction-banner.down {
		box-shadow: 0 0 0 1px rgba(229, 95, 97, 0.08) inset;
	}
	.prediction-banner.neutral {
		box-shadow: 0 0 0 1px rgba(118, 135, 154, 0.08) inset;
	}
	.prediction-details {
		margin-top: 14px;
		padding: 14px 16px;
		border-radius: 18px;
		background: #f7f9fc;
		border: 1px solid #e7edf5;
	}
	.prediction-details summary {
		cursor: pointer;
		font-weight: 700;
		color: #1c4da1;
	}
	.prediction-detail-grid {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 12px;
		margin-top: 14px;
	}
	.prediction-detail-grid div {
		padding: 12px 14px;
		border-radius: 14px;
		background: #ffffff;
		border: 1px solid #e2eaf3;
	}
	.prediction-detail-grid span {
		display: block;
		margin-bottom: 8px;
		color: #748397;
	}
	.prediction-explanation {
		margin-top: 14px;
		color: #5e6c80;
		line-height: 1.6;
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
		.stat-grid, .accuracy-grid { grid-template-columns: 1fr; }
		.prediction-detail-grid { grid-template-columns: 1fr; }
		.panel-head, .feature-head { flex-direction: column; align-items: flex-start; }
	}

	.history-accordion {
		margin-top: 18px;
		background: #fbfdff;
		border: 1px solid #e7edf5;
		border-radius: 18px;
		overflow: hidden;
	}
	.history-accordion summary {
		padding: 16px;
		cursor: pointer;
		font-weight: 600;
		color: #162131;
		list-style: none;
		outline: none;
	}
	.history-accordion summary::-webkit-details-marker {
		display: none;
	}
	.summary-content {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 12px;
	}
	.summary-label {
		color: #748397;
		font-weight: 600;
		text-transform: uppercase;
		font-size: 0.75rem;
		letter-spacing: 0.05em;
	}
	.summary-divider {
		color: #dce6f0;
	}
	.accordion-body {
		padding: 0 16px 16px;
		border-top: 1px solid #e7edf5;
		background: #ffffff;
	}
	.history-table {
		width: 100%;
		border-collapse: collapse;
		text-align: left;
		margin-top: 12px;
	}
	.history-table th {
		padding: 10px 12px;
		color: #748397;
		font-weight: 500;
		border-bottom: 1px solid #e7edf5;
		font-size: 0.9rem;
	}
	.history-table td {
		padding: 12px;
		border-bottom: 1px solid #f0f4f8;
		color: #172233;
		font-size: 0.9rem;
	}
	.badge {
		padding: 4px 8px;
		border-radius: 6px;
		font-size: 0.8rem;
		font-weight: 600;
	}
	.badge-success {
		background: rgba(15, 166, 122, 0.1);
		color: #0fa67a;
	}
	.badge-error {
		background: rgba(229, 95, 97, 0.1);
		color: #e55f61;
	}
</style>
