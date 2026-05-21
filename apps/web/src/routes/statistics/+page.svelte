<script lang="ts">
	import type {
		MarketOverview,
		SymbolStatistics,
		StatisticsOverview,
		HourlyAccuracy,
		PredictionHistoryItem,
		Market
	} from '$lib/types';

	export let data: {
		overview: MarketOverview;
		statistics: StatisticsOverview;
		statisticsMessage: string;
	};

	let assetMeta = new Map<string, { name: string; venue: string; sessionLabel: string }>();
	let homepageCryptoSymbols = new Set<string>();
	let items: SymbolStatistics[] = [];

	let viewModeBySymbol: Record<string, 'all' | 'traded'> = Object.fromEntries(
		items.map((item) => [item.symbol, 'all'])
	);

	$: assetMeta = new Map(
		(data.overview?.assets ?? []).map((asset) => [
			asset.symbol,
			{ name: asset.name, venue: asset.venue, sessionLabel: asset.sessionLabel }
		])
	);

	$: homepageCryptoSymbols = new Set(
		(data.overview?.summaries ?? [])
			.filter((summary) => summary.asset.market === 'crypto')
			.map((summary) => summary.asset.symbol)
	);

	$: items = [...(data.statistics?.items ?? [])]
		.filter((item) => item.market === 'crypto' && homepageCryptoSymbols.has(item.symbol))
		.sort((left, right) => left.symbol.localeCompare(right.symbol));

	$: {
		const nextModes = Object.fromEntries(items.map((item) => [item.symbol, viewModeBySymbol[item.symbol] ?? 'all']));
		viewModeBySymbol = nextModes;
	}

	function setViewMode(symbol: string, mode: 'all' | 'traded') {
		viewModeBySymbol = { ...viewModeBySymbol, [symbol]: mode };
	}

	function viewMode(symbol: string) {
		return viewModeBySymbol[symbol] ?? 'all';
	}

	function formatPercent(rate: number) {
		return `${Math.round(rate * 100)}%`;
	}

	function formatPercentFromRaw(value: number) {
		return `${Math.round(value * 100)}%`;
	}

	function formatNumber(value?: number) {
		if (!value) return '-';
		return new Intl.NumberFormat('tr-TR', { maximumFractionDigits: 2 }).format(value);
	}

	function formatTime(isoString: string) {
		return new Date(isoString).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' });
	}

	function formatDate(isoString: string) {
		return new Date(isoString).toLocaleDateString('tr-TR', { day: '2-digit', month: '2-digit', year: 'numeric' });
	}

	function formatDateTime(isoString?: string) {
		if (!isoString) return '-';
		return new Date(isoString).toLocaleString('tr-TR', {
			day: '2-digit',
			month: '2-digit',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function getDirectionClass(dir: string) {
		if (dir === 'up') return 'text-green';
		if (dir === 'down') return 'text-red';
		return 'text-neutral';
	}

	function getDirectionLabel(dir: string) {
		if (dir === 'up') return 'Yukselis';
		if (dir === 'down') return 'Dusus';
		if (dir === 'neutral') return 'Yatay';
		return '-';
	}

	function predictionActionLabel(dir: string) {
		if (dir === 'up') return 'AL';
		if (dir === 'down') return 'SAT';
		return '-';
	}

	function marketLabel(market: Market) {
		if (market === 'crypto') return 'Kripto';
		if (market === 'bist') return 'BIST';
		return 'ABD Hisse';
	}

	function chartRate(stat: HourlyAccuracy, mode: 'all' | 'traded') {
		return mode === 'all' ? stat.winRate : stat.tradeWinRate;
	}

	function chartCount(stat: HourlyAccuracy, mode: 'all' | 'traded') {
		return mode === 'all' ? stat.total : stat.tradeTotal;
	}

	function chartWins(stat: HourlyAccuracy, mode: 'all' | 'traded') {
		return mode === 'all' ? stat.wins : stat.tradeWins;
	}

	function filteredHistory(history: PredictionHistoryItem[], mode: 'all' | 'traded') {
		const items = mode === 'all' ? history : history.filter((item) => item.tradeAllowed);
		return [...items].reverse();
	}

	function streakLabel(stats: SymbolStatistics) {
		const { currentStreak, streakDirection } = stats.accuracySummary;
		if (!currentStreak || !streakDirection) return 'Seri yok';
		return streakDirection === 'win' ? `${currentStreak} kazanma` : `${currentStreak} kayip`;
	}

	function insightSummary(items: { label: string; winRate: number; tradeWinRate?: number; sampleSize: number; averageVolume?: number }[], metric: 'winRate' | 'tradeWinRate' | 'volume' = 'winRate') {
		if (!items.length) return 'Veri yok';
		return items
			.map((item) => {
				if (metric === 'volume') {
					return `${item.label} (${formatNumber(item.averageVolume)}, n=${item.sampleSize})`;
				}
				const rate = metric === 'tradeWinRate' ? (item.tradeWinRate ?? 0) : item.winRate;
				return `${item.label} (${formatPercent(rate)}, n=${item.sampleSize})`;
			})
			.join(', ');
	}

	function errorAnalysis(hourly: HourlyAccuracy[]) {
		const wrongHours = hourly.filter((h) => h.wrongCount && h.wrongCount > 0);
		if (wrongHours.length === 0) return 'Yeterli hata verisi yok.';
		wrongHours.sort((a, b) => (b.wrongCount || 0) - (a.wrongCount || 0));
		const topWrong = wrongHours[0];
		
		if (!topWrong || !topWrong.wrongCount) return 'Yeterli hata verisi yok.';
		
		const vol = topWrong.avgWrongVolatility || 0;
		const volLabel = vol < 5 ? 'Dusuk' : vol > 15 ? 'Yuksek' : 'Orta';
		
		return `En cok hata ${topWrong.hour.toString().padStart(2, '0')}:00 saatinde (${topWrong.wrongCount} hata). Ort. volatilite skoru: ${vol} (${volLabel}).`;
	}
</script>

<svelte:head>
	<title>Istatistikler | PulseAlpha</title>
</svelte:head>

<div class="shell">
	<header class="masthead">
		<div class="brand">
			<p class="eyebrow">Performans Analizi</p>
			<h1>Kalici sembol istatistikleri ve saatlik detay analizi</h1>
			<p class="intro">
				Site yeniden acildiginda da korunan tahmin gecmisi, sembol bazli performans kartlari ve saatlik isabet davranisini tek ekranda izleyin.
			</p>
		</div>
		<div class="controls">
			<p class="eyebrow">Son guncelleme</p>
			<strong>{formatDateTime(data.statistics?.generatedAt)}</strong>
			<a href="/" class="back-link">Ana sayfaya don</a>
		</div>
	</header>

	{#if data.statisticsMessage}
		<div class="info-banner">{data.statisticsMessage}</div>
	{/if}

	{#if !items.length}
		<div class="empty-state">Kalici istatistik bulunmuyor. Veri olustukca bu ekran dolacak.</div>
	{:else}
		<section class="symbol-grid">
			{#each items as item}
				{@const mode = viewMode(item.symbol)}
				{@const meta = assetMeta.get(item.symbol)}
				<article class="symbol-card">
					<div class="card-head">
						<div>
							<p class="eyebrow">{marketLabel(item.market)}</p>
							<h2>{item.symbol}</h2>
							<p class="meta-line">
								{meta?.name ?? item.symbol}
								{#if meta?.venue}
									<span>{meta.venue}</span>
								{/if}
								{#if meta?.sessionLabel}
									<span>{meta.sessionLabel}</span>
								{/if}
							</p>
						</div>
						<div class="toggle-group">
							<button class:active={mode === 'all'} class="toggle-btn" on:click={() => setViewMode(item.symbol, 'all')}>Tum tahminler</button>
							<button class:active={mode === 'traded'} class="toggle-btn" on:click={() => setViewMode(item.symbol, 'traded')}>Sadece isleme girenler</button>
							<a class="detail-link" href={`/statistics/${item.market}/${item.symbol}`}>Detay sayfasi</a>
						</div>
					</div>

					<div class="kpi-grid">
						<div>
							<span>Genel isabet</span>
							<strong>{formatPercent(item.accuracySummary.winRate)}</strong>
							<p>{item.accuracySummary.sampleSize} cozulmus tahmin</p>
						</div>
						<div>
							<span>Islem isabeti</span>
							<strong>{formatPercent(item.accuracySummary.tradeWinRate)}</strong>
							<p>{item.accuracySummary.tradeSampleSize} islem sinyali</p>
						</div>
						<div>
							<span>Bekleyen tahmin</span>
							<strong>{item.pendingCount}</strong>
							<p>Toplam {item.totalPredictions} kayit</p>
						</div>
						<div>
							<span>Guncel seri</span>
							<strong>{streakLabel(item)}</strong>
							<p>Son hedef: {formatDateTime(item.lastTargetCandleStart)}</p>
						</div>
					</div>

					<div class="section-head">
						<div>
							<p class="eyebrow">Saatlik analiz</p>
							<h3>UTC saatlerine gore performans</h3>
						</div>
						<p class="section-note">Grafik ve tablo UTC saatlerini baz alir.</p>
					</div>

					<div class="bar-chart-container">
						<div class="bar-chart">
							{#each item.hourlyAccuracy as stat}
								{@const rate = chartRate(stat, mode)}
								{@const count = chartCount(stat, mode)}
								{@const wins = chartWins(stat, mode)}
								<div class="bar-col" title={`Saat ${stat.hour.toString().padStart(2, '0')}:00 | İsabet ${count > 0 ? Math.round(rate * 100) : 0}% (${wins}/${count}) | Bekleyen ${stat.pending}`}>
									<div class="bar-fill-bg">
										<div
											class="bar-fill"
											style={`height: ${count > 0 ? rate * 100 : 0}%`}
											class:high={rate >= 0.6}
											class:medium={rate >= 0.4 && rate < 0.6}
											class:low={rate < 0.4 && count > 0}
										></div>
									</div>
									<div class="bar-label">{stat.hour.toString().padStart(2, '0')}</div>
								</div>
							{/each}
						</div>
					</div>

					<div class="insight-grid">
						<div class="insight-card">
							<span>En guclu saatler</span>
							<strong>{insightSummary(item.hourlyInsights.bestHours)}</strong>
						</div>
						<div class="insight-card">
							<span>Zayif saatler</span>
							<strong>{insightSummary(item.hourlyInsights.weakHours)}</strong>
						</div>
						<div class="insight-card highlight-error">
							<span>Hata Analizi (Neden Yanlis?)</span>
							<strong>{errorAnalysis(item.hourlyAccuracy)}</strong>
						</div>
						<div class="insight-card">
							<span>En aktif saatler</span>
							<strong>{insightSummary(item.hourlyInsights.mostActiveHours)}</strong>
						</div>
						<div class="insight-card">
							<span>En iyi islem saatleri</span>
							<strong>{insightSummary(item.hourlyInsights.bestTradeHours, 'tradeWinRate')}</strong>
						</div>
						<div class="insight-card">
							<span>En yuksek hacimli saatler</span>
							<strong>{insightSummary(item.hourlyInsights.highestVolumeHours, 'volume')}</strong>
						</div>
					</div>

					<div class="detail-grid">
						<div class="detail-card">
							<p class="eyebrow">Saat bazli tablo</p>
							<div class="table-container">
								<table class="history-table compact-table">
									<thead>
										<tr>
											<th>Saat</th>
											<th>Toplam</th>
											<th>Isabet</th>
											<th>Islem</th>
											<th>Islem isabeti</th>
											<th>Ort. guven</th>
											<th>Tahmin hacmi</th>
											<th>Gercek hacim</th>
											<th>Zirve hacim</th>
											<th>Bekleyen</th>
										</tr>
									</thead>
									<tbody>
										{#each item.hourlyAccuracy as stat}
											<tr>
												<td>{stat.hour.toString().padStart(2, '0')}:00</td>
												<td>{stat.total}</td>
												<td>{stat.total > 0 ? formatPercent(stat.winRate) : '-'}</td>
												<td>{stat.tradeTotal}</td>
												<td>{stat.tradeTotal > 0 ? formatPercent(stat.tradeWinRate) : '-'}</td>
												<td>{stat.total > 0 ? formatPercentFromRaw(stat.averageConfidence) : '-'}</td>
												<td>{formatNumber(stat.averagePredictionVolume)}</td>
												<td>{formatNumber(stat.averageResolvedVolume)}</td>
												<td>{formatNumber(stat.peakVolume)}</td>
												<td>{stat.pending}</td>
											</tr>
										{/each}
									</tbody>
								</table>
							</div>
						</div>

						<div class="detail-card">
							<p class="eyebrow">Filtre nedenleri</p>
							<div class="filter-list">
								{#if item.tradeFilterBreakdown.length}
									{#each item.tradeFilterBreakdown as filterItem}
										<div class="filter-row">
											<span>{filterItem.reason}</span>
											<strong>{filterItem.count}</strong>
										</div>
									{/each}
								{:else}
									<div class="empty-inline">Filtre kaydi yok.</div>
								{/if}
							</div>

							<div class="status-block">
								<p><strong>Bos kalan saatler:</strong> {item.hourlyInsights.inactiveHours.length ? item.hourlyInsights.inactiveHours.map((hour) => `${hour.toString().padStart(2, '0')}:00`).join(', ') : 'Yok'}</p>
								<p><strong>Bekleyen yogun saatler:</strong> {item.hourlyInsights.pendingHeavyHours.length ? item.hourlyInsights.pendingHeavyHours.map((hour) => `${hour.toString().padStart(2, '0')}:00`).join(', ') : 'Yok'}</p>
								<p><strong>Boga isabeti:</strong> {formatPercent(item.accuracySummary.bullishAccuracy)}</p>
								<p><strong>Ayi isabeti:</strong> {formatPercent(item.accuracySummary.bearishAccuracy)}</p>
								<p><strong>Hacim lider saatler:</strong> {insightSummary(item.hourlyInsights.highestVolumeHours, 'volume')}</p>
							</div>
						</div>
					</div>

					<div class="section-head">
						<div>
							<p class="eyebrow">Gecmis</p>
							<h3>Kalici tahmin kayitlari</h3>
						</div>
						<p class="section-note">{mode === 'all' ? 'Tum kayitlar' : 'Sadece isleme giren kayitlar'}</p>
					</div>

					<div class="table-container">
						{#if filteredHistory(item.history, mode).length > 0}
							<table class="history-table">
								<thead>
									<tr>
										<th>Tarih</th>
										<th>Saat</th>
										<th>Tahmin</th>
										<th>Gerceklesen</th>
										<th>Guven</th>
										<th>Sonuc</th>
										<th>Aksiyon</th>
									</tr>
								</thead>
								<tbody>
									{#each filteredHistory(item.history, mode) as historyItem}
										<tr>
											<td>{formatDate(historyItem.targetCandleStart)}</td>
											<td>{formatTime(historyItem.targetCandleStart)}</td>
											<td><strong class={getDirectionClass(historyItem.predictedDirection)}>{predictionActionLabel(historyItem.predictedDirection)}</strong></td>
											<td>
												{#if historyItem.isPending}
													<span class="text-neutral">Bekliyor</span>
												{:else}
													<strong class={getDirectionClass(historyItem.realizedDirection)}>{getDirectionLabel(historyItem.realizedDirection)}</strong>
												{/if}
											</td>
											<td>{Math.round(historyItem.confidenceScore * 100)}%</td>
											<td>
												{#if historyItem.isPending}
													-
												{:else if historyItem.wasCorrect}
													<span class="badge badge-success">Basarili</span>
												{:else}
													<span class="badge badge-error">Hatali</span>
												{/if}
											</td>
											<td>
												{#if historyItem.tradeAllowed}
													<span class={`action-badge ${historyItem.tradeAction}`}>{historyItem.tradeAction.toUpperCase()}</span>
												{:else}
													<span class="text-neutral">Islem yok</span>
												{/if}
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						{:else}
							<div class="empty-inline">Bu gorunum icin kayit yok.</div>
						{/if}
					</div>
				</article>
			{/each}
		</section>
	{/if}
</div>

<style>
	:global(body) {
		background:
			radial-gradient(circle at 15% 0%, rgba(50, 114, 255, 0.12), transparent 24%),
			radial-gradient(circle at 100% 0%, rgba(7, 163, 127, 0.12), transparent 28%),
			linear-gradient(180deg, #f7fbff 0%, #eef4fb 42%, #e8eef6 100%);
	}

	.shell {
		max-width: 1440px;
		margin: 0 auto;
		padding: 36px 20px 64px;
	}

	.masthead, .section-head, .card-head {
		display: flex;
		justify-content: space-between;
		gap: 20px;
		flex-wrap: wrap;
	}

	.masthead {
		align-items: end;
		margin-bottom: 30px;
	}

	.brand {
		flex: 1 1 720px;
	}

	h1, h2, h3, p, strong {
		margin: 0;
	}

	h1 {
		max-width: 16ch;
		font-size: clamp(2.5rem, 5vw, 4.4rem);
		line-height: 0.95;
		letter-spacing: -0.04em;
	}

	h2 {
		font-size: clamp(1.4rem, 2.4vw, 2rem);
		color: #172233;
	}

	h3 {
		font-size: 1.2rem;
		color: #172233;
	}

	.eyebrow {
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: #748397;
		margin-bottom: 8px;
		display: block;
		font-size: 0.78rem;
	}

	.intro, .meta-line, .section-note, .status-block p {
		color: #5f6e81;
		line-height: 1.6;
	}

	.meta-line {
		display: flex;
		gap: 10px;
		flex-wrap: wrap;
		margin-top: 10px;
	}

	.meta-line span {
		padding-left: 10px;
		border-left: 1px solid #d8e3f0;
	}

	.controls {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 8px;
	}

	.back-link {
		color: #2e7cf6;
		text-decoration: none;
		font-weight: 600;
	}

	.back-link:hover {
		text-decoration: underline;
	}

	.info-banner {
		margin-bottom: 18px;
		padding: 14px 16px;
		border-radius: 14px;
		background: rgba(46, 124, 246, 0.1);
		border: 1px solid rgba(46, 124, 246, 0.16);
		color: #1c4da1;
	}

	.symbol-grid {
		display: grid;
		gap: 24px;
	}

	.symbol-card {
		border: 1px solid rgba(173, 186, 204, 0.24);
		border-radius: 28px;
		background: rgba(255, 255, 255, 0.88);
		box-shadow: 0 20px 50px rgba(110, 133, 160, 0.12);
		backdrop-filter: blur(12px);
		padding: 28px;
	}

	.kpi-grid, .insight-grid, .detail-grid {
		display: grid;
		gap: 14px;
		margin-top: 20px;
	}

	.kpi-grid {
		grid-template-columns: repeat(4, minmax(0, 1fr));
	}

	.insight-grid {
		grid-template-columns: repeat(4, minmax(0, 1fr));
	}

	.detail-grid {
		grid-template-columns: 1.5fr 1fr;
	}

	.kpi-grid div, .insight-card, .detail-card {
		padding: 16px;
		border-radius: 18px;
		background: #f7f9fc;
		border: 1px solid #e7edf5;
	}

	.kpi-grid span, .insight-card span {
		display: block;
		margin-bottom: 8px;
		color: #748397;
	}

	.insight-card.highlight-error {
		background: rgba(229, 95, 97, 0.05);
		border: 1px solid rgba(229, 95, 97, 0.15);
	}

	.insight-card.highlight-error span {
		color: #c94b4d;
	}

	.kpi-grid strong {
		display: block;
		font-size: 1.5rem;
		color: #172233;
		margin-bottom: 6px;
	}

	.toggle-group {
		display: flex;
		background: #f0f4f8;
		border-radius: 12px;
		padding: 4px;
		gap: 4px;
	}

	.toggle-btn {
		background: transparent;
		border: none;
		padding: 8px 16px;
		border-radius: 8px;
		font-weight: 500;
		color: #5f6e81;
		cursor: pointer;
		transition: all 0.2s;
	}

	.toggle-btn.active {
		background: white;
		color: #2e7cf6;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
	}

	.detail-link {
		display: inline-flex;
		align-items: center;
		padding: 8px 14px;
		border-radius: 8px;
		background: #ffffff;
		color: #172233;
		font-weight: 600;
		border: 1px solid #dbe5ef;
	}

	.bar-chart-container {
		margin-top: 24px;
		height: 240px;
	}

	.bar-chart {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		height: 200px;
		gap: 4px;
		padding-bottom: 30px;
		border-bottom: 1px solid #e7edf5;
	}

	.bar-col {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		height: 100%;
		position: relative;
		cursor: pointer;
	}

	.bar-fill-bg {
		width: 100%;
		height: 100%;
		background: #f0f4f8;
		border-radius: 6px 6px 0 0;
		display: flex;
		align-items: flex-end;
		overflow: hidden;
	}

	.bar-fill {
		width: 100%;
		border-radius: 4px 4px 0 0;
		transition: height 0.5s ease;
	}

	.bar-fill.high {
		background: #0fa67a;
	}

	.bar-fill.medium {
		background: #f59e0b;
	}

	.bar-fill.low {
		background: #e55f61;
	}

	.bar-label {
		position: absolute;
		bottom: -25px;
		font-size: 0.85rem;
		color: #748397;
	}

	.table-container {
		margin-top: 16px;
		overflow-x: auto;
	}

	.history-table {
		width: 100%;
		border-collapse: collapse;
		text-align: left;
	}

	.history-table th {
		padding: 12px 16px;
		color: #748397;
		font-weight: 500;
		border-bottom: 1px solid #e7edf5;
		white-space: nowrap;
	}

	.history-table td {
		padding: 14px 16px;
		border-bottom: 1px solid #f0f4f8;
		color: #172233;
	}

	.compact-table td, .compact-table th {
		padding: 10px 12px;
	}

	.filter-list {
		display: grid;
		gap: 10px;
	}

	.filter-row {
		display: flex;
		justify-content: space-between;
		gap: 16px;
		padding: 12px 14px;
		background: #f7f9fc;
		border: 1px solid #e7edf5;
		border-radius: 14px;
	}

	.status-block {
		margin-top: 18px;
		display: grid;
		gap: 8px;
	}

	.text-green {
		color: #0fa67a;
	}

	.text-red {
		color: #e55f61;
	}

	.text-neutral {
		color: #748397;
	}

	.badge, .action-badge {
		padding: 6px 10px;
		border-radius: 6px;
		font-size: 0.85rem;
		font-weight: 600;
	}

	.badge-success, .action-badge.buy {
		background: rgba(15, 166, 122, 0.1);
		color: #0fa67a;
	}

	.badge-error, .action-badge.sell {
		background: rgba(229, 95, 97, 0.1);
		color: #e55f61;
	}

	.action-badge.hold, .action-badge.no_trade {
		background: rgba(245, 158, 11, 0.1);
		color: #f59e0b;
	}

	.empty-state, .empty-inline {
		text-align: center;
		padding: 24px;
		color: #748397;
		background: #f7f9fc;
		border-radius: 12px;
	}

	@media (max-width: 1080px) {
		.kpi-grid, .insight-grid, .detail-grid {
			grid-template-columns: 1fr 1fr;
		}
	}

	@media (max-width: 760px) {
		.shell {
			padding-inline: 14px;
		}

		.kpi-grid, .insight-grid, .detail-grid {
			grid-template-columns: 1fr;
		}

		.toggle-group {
			width: 100%;
		}

		.toggle-btn {
			flex: 1;
		}
	}
</style>
