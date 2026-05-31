<script lang="ts">
	import type { AssetDetail, HourlyAccuracy, HourlyInsight, SymbolStatistics } from '$lib/types';

	export let item: SymbolStatistics | null = null;
	export let detail: AssetDetail;
	export let focusTimestamp: string | null = null;

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

	function formatTime(isoString: string, timeZone = 'Europe/Istanbul') {
		return new Date(isoString).toLocaleTimeString('tr-TR', {
			hour: '2-digit',
			minute: '2-digit',
			timeZone
		});
	}

	function formatDate(isoString: string) {
		return new Date(isoString).toLocaleDateString('tr-TR', {
			day: '2-digit',
			month: '2-digit',
			year: 'numeric'
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

	function chartRate(stat: HourlyAccuracy, mode: 'all' | 'traded') {
		return mode === 'all' ? stat.winRate : stat.tradeWinRate;
	}

	function chartCount(stat: HourlyAccuracy, mode: 'all' | 'traded') {
		return mode === 'all' ? stat.total : stat.tradeTotal;
	}

	function chartWins(stat: HourlyAccuracy, mode: 'all' | 'traded') {
		return mode === 'all' ? stat.wins : stat.tradeWins;
	}

	function filteredHistory(mode: 'all' | 'traded') {
		if (!item) return [];
		const items = mode === 'all' ? item.history : item.history.filter((entry) => entry.tradeAllowed);
		return [...items].reverse();
	}

	function streakLabel() {
		if (!item) return 'Seri yok';
		const { currentStreak, streakDirection } = item.accuracySummary;
		if (!currentStreak || !streakDirection) return 'Seri yok';
		return streakDirection === 'win' ? `${currentStreak} kazanma` : `${currentStreak} kayip`;
	}

	function insightSummary(
		items: HourlyInsight[],
		metric: 'winRate' | 'tradeWinRate' | 'volume' = 'winRate'
	) {
		if (!items.length) return 'Veri yok';
		return items
			.map((entry) => {
				if (metric === 'volume') {
					return `${hourLabelTR(entry.hour)} (ort. ${formatNumber(entry.averageVolume)}, n=${entry.sampleSize})`;
				}
				const rate = metric === 'tradeWinRate' ? entry.tradeWinRate : entry.winRate;
				return `${hourLabelTR(entry.hour)} (${formatPercent(rate || 0)}, n=${entry.sampleSize})`;
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
		return `En cok hata ${hourLabelTR(topWrong.hour)} diliminde (${topWrong.wrongCount} hata). Ort. volatilite skoru ${vol} (${volLabel}).`;
	}

	function hourLabelUTC(hour: number) {
		return `${hour.toString().padStart(2, '0')}:00`;
	}

	function hourLabelTR(hour: number) {
		const base = new Date(Date.UTC(2026, 0, 1, hour, 0, 0));
		return base.toLocaleTimeString('tr-TR', {
			hour: '2-digit',
			minute: '2-digit',
			timeZone: 'Europe/Istanbul'
		});
	}

	function candleLink(timestamp: string) {
		return `/markets/${detail.asset.market}/${detail.asset.symbol}?focus=${encodeURIComponent(timestamp)}`;
	}

	let viewMode: 'all' | 'traded' = 'all';
</script>

{#if item}
<div class="stats-panel-wrapper">
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
			<strong>{streakLabel()}</strong>
			<p>Son 5 pencere: {formatPercent(item.accuracySummary.recentWindowWinRate)}</p>
		</div>
	</div>

	<section class="section-head">
		<div>
			<p class="eyebrow">Saatlik analiz</p>
			<h3>Hangi saatlerde daha dogru?</h3>
		</div>
		<div class="toggle-group">
			<button class:active={viewMode === 'all'} class="toggle-btn" on:click={() => (viewMode = 'all')}>
				Tum tahminler
			</button>
			<button class:active={viewMode === 'traded'} class="toggle-btn" on:click={() => (viewMode = 'traded')}>
				Sadece isleme girenler
			</button>
		</div>
	</section>

	<div class="bar-chart-container">
		<div class="bar-chart">
			{#each item.hourlyAccuracy as stat}
				{@const rate = chartRate(stat, viewMode)}
				{@const count = chartCount(stat, viewMode)}
				{@const wins = chartWins(stat, viewMode)}
				<div
					class="bar-col"
					title={`TR ${hourLabelTR(stat.hour)} | UTC ${hourLabelUTC(stat.hour)} | Isabet ${count > 0 ? Math.round(rate * 100) : 0}% (${wins}/${count}) | Bekleyen ${stat.pending}`}
				>
					<div class="bar-fill-bg">
						<div
							class="bar-fill"
							style={`height: ${count > 0 ? rate * 100 : 0}%`}
							class:high={rate >= 0.6}
							class:medium={rate >= 0.4 && rate < 0.6}
							class:low={rate < 0.4 && count > 0}
						></div>
					</div>
					<div class="bar-label">{hourLabelTR(stat.hour)}</div>
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
		<div class="insight-card">
			<span>En iyi islem saatleri</span>
			<strong>{insightSummary(item.hourlyInsights.bestTradeHours, 'tradeWinRate')}</strong>
		</div>
		<div class="insight-card">
			<span>En yuksek hacimli saatler</span>
			<strong>{insightSummary(item.hourlyInsights.highestVolumeHours, 'volume')}</strong>
		</div>
		<div class="insight-card">
			<span>En aktif saatler</span>
			<strong>{insightSummary(item.hourlyInsights.mostActiveHours)}</strong>
		</div>
		<div class="insight-card highlight-error">
			<span>Hata analizi</span>
			<strong>{errorAnalysis(item.hourlyAccuracy)}</strong>
		</div>
	</div>

	<div class="detail-grid">
		<div class="detail-card">
			<p class="eyebrow">Saat bazli tablo</p>
			<div class="table-container">
				<table class="history-table compact-table">
					<thead>
						<tr>
							<th>TR</th>
							<th>UTC</th>
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
								<td>{hourLabelTR(stat.hour)}</td>
								<td>{hourLabelUTC(stat.hour)}</td>
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
			<p class="eyebrow">Filtre ve davranis ozetleri</p>
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
				<p><strong>Boga isabeti:</strong> {formatPercent(item.accuracySummary.bullishAccuracy)}</p>
				<p><strong>Ayi isabeti:</strong> {formatPercent(item.accuracySummary.bearishAccuracy)}</p>
				<p><strong>Bos kalan saatler:</strong> {item.hourlyInsights.inactiveHours.length ? item.hourlyInsights.inactiveHours.map((hour) => hourLabelTR(hour)).join(', ') : 'Yok'}</p>
				<p><strong>Bekleyen yogun saatler:</strong> {item.hourlyInsights.pendingHeavyHours.length ? item.hourlyInsights.pendingHeavyHours.map((hour) => hourLabelTR(hour)).join(', ') : 'Yok'}</p>
			</div>
		</div>
	</div>

	<section class="section-head">
		<div>
			<p class="eyebrow">Recovery Analizi</p>
			<h3>Ardisik islem dogrulugu (Martingale)</h3>
		</div>
		<p class="section-note">Kaybettikten sonra girilen yeni islemlerin kacinci adimda basariya ulastigini gosterir. (Sadece isleme girenler baz alinir)</p>
	</section>

	<div class="table-container">
		<table class="history-table compact-table">
			<thead>
				<tr>
					<th>Adim (Islem Sirasi)</th>
					<th>Deneme Sayisi</th>
					<th>Adim Basarisi</th>
					<th>Adim Dogruluk Orani</th>
					<th>Kumulatif Basari</th>
					<th>Kumulatif Dogruluk Orani</th>
				</tr>
			</thead>
			<tbody>
				{#if item.accuracySummary.recoverySteps && item.accuracySummary.recoverySteps.length > 0}
					{#each item.accuracySummary.recoverySteps as step}
						<tr>
							<td>{step.stepNumber}. Islem</td>
							<td>{step.attempts}</td>
							<td>{step.wins}</td>
							<td><strong>{formatPercent(step.stepWinRate)}</strong></td>
							<td>{step.cumulativeWins}</td>
							<td><strong class="text-green">{formatPercent(step.cumulativeRate)}</strong></td>
						</tr>
					{/each}
				{:else}
					<tr>
						<td colspan="6" class="text-center">Henuz yeterli islem verisi yok.</td>
					</tr>
				{/if}
			</tbody>
		</table>
	</div>

	<div class="table-container">
		<table class="history-table compact-table">
			<thead>
				<tr>
					<th>Adim</th>
					<th>Tarih</th>
					<th>Saat</th>
					<th>Yon</th>
					<th>Gercek</th>
					<th>Guven</th>
					<th>Odak</th>
				</tr>
			</thead>
			<tbody>
				{#if item.accuracySummary.recoveryWrongCandles && item.accuracySummary.recoveryWrongCandles.length > 0}
					{#each item.accuracySummary.recoveryWrongCandles as candle}
						<tr class="wrong-candle-row">
							<td>{candle.stepNumber}. Islem</td>
							<td>{formatDate(candle.targetCandleStart)}</td>
							<td>{formatTime(candle.targetCandleStart)}</td>
							<td><strong class={getDirectionClass(candle.predictedDirection)}>{predictionActionLabel(candle.predictedDirection)}</strong></td>
							<td><strong class={getDirectionClass(candle.realizedDirection)}>{getDirectionLabel(candle.realizedDirection)}</strong></td>
							<td>{Math.round(candle.confidenceScore * 100)}%</td>
							<td><a class="focus-link" href={candleLink(candle.targetCandleStart)}>Mumu ac</a></td>
						</tr>
					{/each}
				{:else}
					<tr>
						<td colspan="7" class="text-center">7. islem ve sonrasindaki hatali mum kaydi yok.</td>
					</tr>
				{/if}
			</tbody>
		</table>
	</div>

	<section class="section-head">
		<div>
			<p class="eyebrow">Gecmis</p>
			<h3>Kalici tahmin kayitlari</h3>
		</div>
		<p class="section-note">{viewMode === 'all' ? 'Tum kayitlar' : 'Sadece isleme giren kayitlar'}</p>
	</section>

	<div class="table-container">
		{#if filteredHistory(viewMode).length > 0}
			<table class="history-table">
				<thead>
					<tr>
						<th>Tarih</th>
						<th>TR Saat</th>
						<th>UTC Saat</th>
						<th>Tahmin</th>
						<th>Gerceklesen</th>
						<th>Guven</th>
						<th>Sonuc</th>
						<th>Aksiyon</th>
					</tr>
				</thead>
				<tbody>
					{#each filteredHistory(viewMode) as historyItem}
						<tr>
							<td>{formatDate(historyItem.targetCandleStart)}</td>
							<td>{formatTime(historyItem.targetCandleStart, 'Europe/Istanbul')}</td>
							<td>{formatTime(historyItem.targetCandleStart, 'UTC')}</td>
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
</div>
{:else}
<div class="empty-inline">Sembol istatistikleri yuklenemedi veya henuz hazir degil.</div>
{/if}

<style>
	.stats-panel-wrapper {
		margin-top: 24px;
		display: grid;
		gap: 20px;
	}

	.section-head {
		display: flex;
		justify-content: space-between;
		gap: 20px;
		flex-wrap: wrap;
		align-items: center;
		margin-top: 10px;
	}

	.kpi-grid {
		display: grid;
		gap: 16px;
		grid-template-columns: repeat(4, minmax(0, 1fr));
	}

	.insight-grid {
		display: grid;
		gap: 16px;
		grid-template-columns: repeat(3, minmax(0, 1fr));
	}

	.detail-grid {
		display: grid;
		gap: 16px;
		grid-template-columns: 1.6fr 1fr;
	}

	h3, p, strong {
		margin: 0;
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

	.kpi-grid div, .insight-card, .detail-card {
		padding: 20px;
		border-radius: 22px;
		background: rgba(255, 255, 255, 0.9);
		border: 1px solid rgba(173, 186, 204, 0.24);
		box-shadow: 0 18px 42px rgba(110, 133, 160, 0.12);
	}

	.section-note, .status-block p {
		color: #5f6e81;
		line-height: 1.6;
	}

	.kpi-grid span, .insight-card span {
		display: block;
		margin-bottom: 8px;
		color: #748397;
	}

	.kpi-grid strong {
		display: block;
		font-size: 1.5rem;
		color: #172233;
		margin-bottom: 6px;
	}
	
	.kpi-grid p {
		color: #5f6e81;
		font-size: 0.9rem;
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
		font-weight: 600;
		color: #5f6e81;
		cursor: pointer;
	}

	.toggle-btn.active {
		background: white;
		color: #2e7cf6;
		box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
	}

	.bar-chart-container {
		height: 240px;
		padding: 18px;
		border-radius: 22px;
		background: rgba(255, 255, 255, 0.9);
		border: 1px solid rgba(173, 186, 204, 0.24);
		box-shadow: 0 18px 42px rgba(110, 133, 160, 0.12);
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
		transition: height 0.4s ease;
	}

	.bar-fill.high { background: #0fa67a; }
	.bar-fill.medium { background: #f59e0b; }
	.bar-fill.low { background: #e55f61; }

	.bar-label {
		position: absolute;
		bottom: -25px;
		font-size: 0.8rem;
		color: #748397;
	}

	.insight-card.highlight-error {
		background: rgba(229, 95, 97, 0.05);
		border-color: rgba(229, 95, 97, 0.16);
	}

	.insight-card.highlight-error span {
		color: #c94b4d;
	}

	.table-container {
		margin-top: 14px;
		overflow-x: auto;
		background: rgba(255, 255, 255, 0.9);
		border-radius: 22px;
		border: 1px solid rgba(173, 186, 204, 0.24);
		box-shadow: 0 18px 42px rgba(110, 133, 160, 0.12);
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

	.filter-list { display: grid; gap: 10px; }
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

	.text-green { color: #0fa67a; }
	.text-red { color: #e55f61; }
	.text-neutral { color: #748397; }

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

	.empty-inline {
		text-align: center;
		padding: 24px;
		color: #748397;
		background: #f7f9fc;
		border-radius: 12px;
		border: 1px solid rgba(173, 186, 204, 0.24);
	}

	@media (max-width: 1080px) {
		.kpi-grid, .insight-grid, .detail-grid {
			grid-template-columns: 1fr 1fr;
		}
	}

	.focus-link {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 6px 10px;
		border-radius: 999px;
		background: rgba(46, 124, 246, 0.08);
		color: #1c4da1;
		font-weight: 700;
		white-space: nowrap;
	}

	.wrong-candle-row {
		background: rgba(229, 95, 97, 0.04);
	}

	@media (max-width: 760px) {
		.kpi-grid, .insight-grid, .detail-grid {
			grid-template-columns: 1fr;
		}
		.toggle-group { width: 100%; }
		.toggle-btn { flex: 1; }
	}
</style>
