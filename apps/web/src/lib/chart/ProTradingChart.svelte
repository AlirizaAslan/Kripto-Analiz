<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import {
		CandlestickSeries,
		ColorType,
		createChart,
		CrosshairMode,
		HistogramSeries,
		LineSeries,
		LineStyle
	} from 'lightweight-charts';
	import type { Time } from 'lightweight-charts';
	import type { AssetDetail } from '$lib/types';
	import { movingAverage, normalizeCandles, type ChartPoint } from './chart-data';
	import { CHART_TIMEFRAMES, PRO_CHART_SERIES, PRO_CHART_THEME, type ChartTimeframe } from './pro-chart-theme';

	export let detail: AssetDetail;

	let chartHost: HTMLDivElement;
	let chart: ReturnType<typeof createChart> | undefined;
	let candleSeries: any;
	let volumeSeries: any;
	let maSeries: any[] = [];
	let volumeMaSeries: any[] = [];
	let resizeObserver: ResizeObserver | undefined;
	let lastPriceLine: any;
	let activeTimeframe: ChartTimeframe = '1m';
	let selectedCandle: ChartPoint | null = null;
	let mounted = false;
	let barSpacing = 9;
	let lastSyncedTimeframe: ChartTimeframe | undefined;

	const minBarSpacing = 4;
	const maxBarSpacing = 36;
	const zoomStep = 4;
	const chartTimeZone = 'Europe/Istanbul';
	const shortTimeFormatter = new Intl.DateTimeFormat('tr-TR', {
		timeZone: chartTimeZone,
		hour: '2-digit',
		minute: '2-digit'
	});
	const shortDateFormatter = new Intl.DateTimeFormat('tr-TR', {
		timeZone: chartTimeZone,
		day: '2-digit',
		month: '2-digit'
	});
	const candleTimeFormatter = new Intl.DateTimeFormat('tr-TR', {
		timeZone: chartTimeZone,
		day: '2-digit',
		month: '2-digit',
		hour: '2-digit',
		minute: '2-digit'
	});

	function formatPrice(value: number) {
		return new Intl.NumberFormat('en-US', {
			maximumFractionDigits: value >= 100 ? 2 : 6
		}).format(value);
	}

	function formatCompact(value: number) {
		return new Intl.NumberFormat('en-US', {
			notation: 'compact',
			maximumFractionDigits: 2
		}).format(value);
	}

	function formatSigned(value: number) {
		const prefix = value > 0 ? '+' : '';
		return `${prefix}${formatPrice(value)}`;
	}

	function formatPercent(open: number, close: number) {
		if (!open) return '0.00%';
		const value = ((close - open) / open) * 100;
		return `${value > 0 ? '+' : ''}${value.toFixed(2)}%`;
	}

	function formatTimestamp(timestamp: string) {
		const date = new Date(timestamp);
		if (Number.isNaN(date.getTime())) return '';
		return candleTimeFormatter.format(date);
	}

	function dateFromChartTime(time: Time) {
		if (typeof time === 'number') {
			return new Date(time * 1000);
		}
		if (typeof time === 'string') {
			return new Date(time);
		}
		return new Date(Date.UTC(time.year, time.month - 1, time.day));
	}

	function formatChartTime(time: Time) {
		const date = dateFromChartTime(time);
		if (Number.isNaN(date.getTime())) return '';
		return candleTimeFormatter.format(date);
	}

	function formatTickTime(time: Time) {
		const date = dateFromChartTime(time);
		if (Number.isNaN(date.getTime())) return '';
		return activeTimeframe === '1d' || activeTimeframe === '1w'
			? shortDateFormatter.format(date)
			: shortTimeFormatter.format(date);
	}

	function candlesForTimeframe(timeframe: ChartTimeframe) {
		const config = CHART_TIMEFRAMES.find((item) => item.key === timeframe);
		const chartSets = detail.chartCandles ?? {};
		return (
			chartSets[config?.candleKey ?? timeframe] ??
			chartSets[timeframe] ??
			chartSets['1m'] ??
			chartSets['1m_recent'] ??
			detail.candles ??
			[]
		);
	}

	function currentPoints() {
		return normalizeCandles(candlesForTimeframe(activeTimeframe));
	}

	function buildChart() {
		if (!chartHost) return;
		chart = createChart(chartHost, {
			autoSize: true,
			layout: {
				background: { type: ColorType.Solid, color: PRO_CHART_THEME.background },
				textColor: PRO_CHART_THEME.textMuted,
				fontFamily: '"Aptos", "Segoe UI Variable Display", "Inter", sans-serif',
				fontSize: 12,
				attributionLogo: false
			},
			grid: {
				vertLines: { color: PRO_CHART_THEME.gridSoft },
				horzLines: { color: PRO_CHART_THEME.grid }
			},
			crosshair: {
				mode: CrosshairMode.Normal,
				vertLine: {
					color: 'rgba(226, 232, 240, 0.36)',
					width: 1,
					style: LineStyle.Dashed,
					labelBackgroundColor: PRO_CHART_THEME.panelElevated
				},
				horzLine: {
					color: 'rgba(226, 232, 240, 0.36)',
					width: 1,
					style: LineStyle.Dashed,
					labelBackgroundColor: PRO_CHART_THEME.panelElevated
				}
			},
			rightPriceScale: {
				borderColor: PRO_CHART_THEME.border,
				scaleMargins: { top: 0.08, bottom: 0.28 }
			},
			timeScale: {
				borderColor: PRO_CHART_THEME.border,
				timeVisible: true,
				secondsVisible: activeTimeframe === '1m',
				barSpacing,
				minBarSpacing,
				tickMarkFormatter: (time: Time) => formatTickTime(time)
			},
			localization: {
				priceFormatter: (price: number) => formatPrice(price),
				timeFormatter: (time: Time) => formatChartTime(time)
			},
			handleScroll: {
				mouseWheel: true,
				pressedMouseMove: true,
				horzTouchDrag: true,
				vertTouchDrag: false
			},
			handleScale: {
				axisPressedMouseMove: true,
				mouseWheel: true,
				pinch: true
			}
		});

		candleSeries = chart.addSeries(CandlestickSeries, {
			upColor: PRO_CHART_THEME.bullish,
			downColor: PRO_CHART_THEME.bearish,
			borderUpColor: PRO_CHART_THEME.bullish,
			borderDownColor: PRO_CHART_THEME.bearish,
			wickUpColor: PRO_CHART_THEME.bullish,
			wickDownColor: PRO_CHART_THEME.bearish,
			priceLineVisible: false
		});

		volumeSeries = chart.addSeries(HistogramSeries, {
			priceFormat: { type: 'volume' },
			priceScaleId: '',
			base: 0
		});
		chart.priceScale('').applyOptions({
			scaleMargins: { top: 0.78, bottom: 0 }
		});

		maSeries = [
			chart.addSeries(LineSeries, { color: PRO_CHART_THEME.ma7, lineWidth: 1, priceLineVisible: false, lastValueVisible: false }),
			chart.addSeries(LineSeries, { color: PRO_CHART_THEME.ma25, lineWidth: 1, priceLineVisible: false, lastValueVisible: false }),
			chart.addSeries(LineSeries, { color: PRO_CHART_THEME.ma99, lineWidth: 1, priceLineVisible: false, lastValueVisible: false })
		];

		volumeMaSeries = [
			chart.addSeries(LineSeries, {
				color: PRO_CHART_THEME.volumeMaFast,
				lineWidth: 1,
				priceScaleId: '',
				priceLineVisible: false,
				lastValueVisible: false
			}),
			chart.addSeries(LineSeries, {
				color: PRO_CHART_THEME.volumeMaSlow,
				lineWidth: 1,
				priceScaleId: '',
				priceLineVisible: false,
				lastValueVisible: false
			})
		];

		chart.subscribeCrosshairMove((param: any) => {
			const points = currentPoints();
			if (!param?.time || !points.length) {
				selectedCandle = points[points.length - 1] ?? null;
				return;
			}
			const hoveredTime = Number(param.time);
			selectedCandle = points.find((point) => point.time === hoveredTime) ?? points[points.length - 1] ?? null;
		});

		resizeObserver = new ResizeObserver(() => chart?.applyOptions({ autoSize: true }));
		resizeObserver.observe(chartHost);
	}

	function applyZoom(nextSpacing: number) {
		if (!chart) return;
		barSpacing = Math.min(maxBarSpacing, Math.max(minBarSpacing, nextSpacing));
		chart.timeScale().applyOptions({ barSpacing });
	}

	function zoomIn() {
		applyZoom(barSpacing + zoomStep);
	}

	function zoomOut() {
		applyZoom(barSpacing - zoomStep);
	}

	function fitChart() {
		if (!chart) return;
		barSpacing = 9;
		chart.timeScale().applyOptions({ barSpacing });
		chart.timeScale().fitContent();
	}

	function syncChart() {
		if (!chart || !candleSeries || !volumeSeries) return;
		const points = currentPoints();
		const shouldFit = lastSyncedTimeframe !== activeTimeframe;
		selectedCandle = selectedCandle && points.some((point) => point.time === selectedCandle?.time) ? selectedCandle : points[points.length - 1] ?? null;

		candleSeries.setData(
			points.map((point) => ({
				time: point.time,
				open: point.open,
				high: point.high,
				low: point.low,
				close: point.close
			}))
		);

		volumeSeries.setData(
			points.map((point) => ({
				time: point.time,
				value: point.volume,
				color: point.direction === 'up' ? 'rgba(8, 153, 129, 0.42)' : 'rgba(242, 54, 69, 0.42)'
			}))
		);

		for (const [index, period] of PRO_CHART_SERIES.maPeriods.entries()) {
			maSeries[index]?.setData(movingAverage(points, period));
		}
		for (const [index, period] of PRO_CHART_SERIES.volumeMaPeriods.entries()) {
			volumeMaSeries[index]?.setData(movingAverage(points, period, 'volume'));
		}

		if (lastPriceLine) {
			candleSeries.removePriceLine(lastPriceLine);
			lastPriceLine = undefined;
		}
		const last = points[points.length - 1];
		if (last) {
			const color = last.direction === 'up' ? PRO_CHART_THEME.bullish : PRO_CHART_THEME.bearish;
			lastPriceLine = candleSeries.createPriceLine({
				price: last.close,
				color,
				lineWidth: 1,
				lineStyle: LineStyle.Solid,
				axisLabelVisible: true,
				title: 'LAST'
			});
		}
		chart.applyOptions({
			timeScale: { secondsVisible: activeTimeframe === '1m' }
		});
		if (shouldFit) {
			chart.timeScale().applyOptions({ barSpacing });
			chart.timeScale().fitContent();
			lastSyncedTimeframe = activeTimeframe;
		}
	}

	onMount(() => {
		buildChart();
		mounted = true;
		syncChart();
	});

	onDestroy(() => {
		resizeObserver?.disconnect();
		chart?.remove();
	});

	$: if (mounted && detail && activeTimeframe) {
		syncChart();
	}

	$: activePoints = currentPoints();
	$: infoCandle = selectedCandle ?? activePoints[activePoints.length - 1] ?? null;
	$: lastDirection = activePoints[activePoints.length - 1]?.direction ?? 'up';
</script>

<section class="terminal">
	<div class="terminal-top">
		<div class="tab-strip" aria-label="Chart sections">
			<button class="tab active" type="button">Chart</button>
			<button class="tab" type="button">Info</button>
			<button class="tab" type="button">Trading Data</button>
		</div>
		<div class="tool-strip" aria-label="Chart tools">
			<button type="button" title="Indicators">MA</button>
			<button type="button" title="Zoom out" on:click={zoomOut}>-</button>
			<button type="button" title="Zoom in" on:click={zoomIn}>+</button>
			<button type="button" title="Fit chart" on:click={fitChart}>[]</button>
		</div>
	</div>

	<div class="market-bar">
		<div>
			<span class="symbol">{detail.asset.symbol}</span>
			<span class="venue">{detail.asset.venue} / {detail.asset.instrumentType}</span>
		</div>
		<div class="last-price" class:up={lastDirection === 'up'} class:down={lastDirection === 'down'}>
			{formatPrice(detail.asset.lastPrice)}
		</div>
		<div class="timeframes" aria-label="Timeframe switcher">
			{#each CHART_TIMEFRAMES as timeframe}
				<button
					type="button"
					class:active={activeTimeframe === timeframe.key}
					on:click={() => (activeTimeframe = timeframe.key)}
				>
					{timeframe.label}
				</button>
			{/each}
		</div>
	</div>

	<div class="info-bar">
		{#if infoCandle}
			<span>{formatTimestamp(infoCandle.timestamp)}</span>
			<span>O <strong>{formatPrice(infoCandle.open)}</strong></span>
			<span>H <strong>{formatPrice(infoCandle.high)}</strong></span>
			<span>L <strong>{formatPrice(infoCandle.low)}</strong></span>
			<span>C <strong>{formatPrice(infoCandle.close)}</strong></span>
			<span class:up={infoCandle.change >= 0} class:down={infoCandle.change < 0}>Chg <strong>{formatSigned(infoCandle.change)} ({formatPercent(infoCandle.open, infoCandle.close)})</strong></span>
			<span>Range <strong>{formatPrice(infoCandle.range)}</strong></span>
			<span>Vol <strong>{formatCompact(infoCandle.volume)}</strong></span>
		{:else}
			<span>Chart data is waiting for the next market snapshot.</span>
		{/if}
	</div>

	<div class="chart-wrap">
		<div bind:this={chartHost} class="chart-host" aria-label={`${detail.asset.symbol} professional candlestick chart`}></div>
		<div class="legend">
			<span><i style={`background:${PRO_CHART_THEME.ma7}`}></i>MA(7)</span>
			<span><i style={`background:${PRO_CHART_THEME.ma25}`}></i>MA(25)</span>
			<span><i style={`background:${PRO_CHART_THEME.ma99}`}></i>MA(99)</span>
			<span><i style={`background:${PRO_CHART_THEME.volumeMaFast}`}></i>Vol MA</span>
		</div>
	</div>

	<div class="terminal-bottom" aria-label="Market mode navigation">
		<button class="active" type="button">Spot</button>
		<button type="button">Cross</button>
		<button type="button">Isolated</button>
		<button type="button">Grid</button>
		<span>{detail.disclaimer}</span>
	</div>
</section>

<style>
	.terminal {
		--panel: #ffffff;
		--panel-soft: #f6f8fb;
		--border: rgba(100, 116, 139, 0.2);
		--text: #111827;
		--muted: #475569;
		--dim: #64748b;
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: #ffffff;
		box-shadow: 0 18px 48px rgba(15, 23, 42, 0.1);
		color: var(--text);
	}

	button {
		font: inherit;
		cursor: pointer;
	}

	.terminal-top,
	.market-bar,
	.info-bar,
	.terminal-bottom {
		display: flex;
		align-items: center;
		gap: 12px;
		border-bottom: 1px solid var(--border);
		background: rgba(248, 250, 252, 0.96);
	}

	.terminal-top {
		justify-content: space-between;
		min-height: 42px;
		padding: 0 12px;
	}

	.tab-strip,
	.tool-strip,
	.timeframes,
	.terminal-bottom {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.tab,
	.tool-strip button,
	.timeframes button,
	.terminal-bottom button {
		border: 1px solid transparent;
		border-radius: 6px;
		background: transparent;
		color: var(--muted);
		line-height: 1;
		transition:
			background 140ms ease,
			border-color 140ms ease,
			color 140ms ease;
	}

	.tab {
		padding: 10px 12px;
	}

	.tab.active,
	.timeframes button.active,
	.terminal-bottom button.active {
		border-color: rgba(37, 99, 235, 0.24);
		background: rgba(37, 99, 235, 0.1);
		color: #1d4ed8;
	}

	.tool-strip button {
		width: 30px;
		height: 28px;
	}

	.tool-strip button:hover,
	.timeframes button:hover,
	.terminal-bottom button:hover {
		background: rgba(100, 116, 139, 0.1);
		color: var(--text);
	}

	.market-bar {
		justify-content: space-between;
		min-height: 58px;
		padding: 10px 12px;
		flex-wrap: wrap;
	}

	.symbol {
		display: block;
		font-size: 1.05rem;
		font-weight: 700;
		letter-spacing: 0;
	}

	.venue {
		display: block;
		margin-top: 4px;
		color: var(--dim);
		font-size: 0.75rem;
		text-transform: uppercase;
	}

	.last-price {
		padding: 8px 10px;
		border-radius: 6px;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
	}

	.last-price.up {
		background: rgba(8, 153, 129, 0.12);
		color: #047857;
	}

	.last-price.down {
		background: rgba(242, 54, 69, 0.12);
		color: #dc2626;
	}

	.timeframes button {
		min-width: 38px;
		height: 30px;
		padding: 0 9px;
		font-size: 0.78rem;
		font-weight: 700;
	}

	.info-bar {
		min-height: 38px;
		padding: 8px 12px;
		flex-wrap: wrap;
		color: var(--muted);
		font-size: 0.78rem;
		font-variant-numeric: tabular-nums;
	}

	.info-bar strong {
		color: var(--text);
		font-weight: 600;
	}

	.info-bar .up strong,
	.info-bar .up {
		color: #047857;
	}

	.info-bar .down strong,
	.info-bar .down {
		color: #dc2626;
	}

	.chart-wrap {
		position: relative;
		background: #ffffff;
	}

	.chart-host {
		min-height: 520px;
		width: 100%;
	}

	.legend {
		position: absolute;
		left: 14px;
		top: 12px;
		display: flex;
		gap: 12px;
		flex-wrap: wrap;
		color: var(--muted);
		font-size: 0.74rem;
		pointer-events: none;
	}

	.legend span {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 6px;
		border-radius: 4px;
		background: rgba(255, 255, 255, 0.86);
		box-shadow: 0 1px 3px rgba(15, 23, 42, 0.08);
	}

	.legend i {
		display: inline-block;
		width: 18px;
		height: 2px;
		border-radius: 999px;
	}

	.terminal-bottom {
		border-top: 1px solid var(--border);
		border-bottom: 0;
		min-height: 42px;
		padding: 7px 12px;
	}

	.terminal-bottom button {
		height: 28px;
		padding: 0 10px;
		font-size: 0.78rem;
		font-weight: 700;
	}

	.terminal-bottom span {
		margin-left: auto;
		color: var(--dim);
		font-size: 0.72rem;
		line-height: 1.35;
		text-align: right;
	}

	@media (max-width: 760px) {
		.terminal-top {
			align-items: stretch;
			flex-direction: column;
			padding: 8px;
		}

		.market-bar {
			align-items: flex-start;
			flex-direction: column;
		}

		.timeframes {
			width: 100%;
			overflow-x: auto;
			padding-bottom: 2px;
		}

		.chart-host {
			min-height: 420px;
		}

		.legend {
			position: static;
			padding: 10px 12px 0;
			background: #ffffff;
		}

		.terminal-bottom {
			align-items: flex-start;
			flex-wrap: wrap;
		}

		.terminal-bottom span {
			width: 100%;
			margin-left: 0;
			text-align: left;
		}
	}
</style>
