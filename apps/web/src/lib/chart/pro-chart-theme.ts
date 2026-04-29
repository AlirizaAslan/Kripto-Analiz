export type ChartTimeframe = '1m' | '5m' | '15m' | '1h' | '4h' | '1d' | '1w';

export const CHART_TIMEFRAMES: Array<{ key: ChartTimeframe; label: string; candleKey: string }> = [
	{ key: '1m', label: '1m', candleKey: '1m' },
	{ key: '5m', label: '5m', candleKey: '5m' },
	{ key: '15m', label: '15m', candleKey: '15m' },
	{ key: '1h', label: '1H', candleKey: '1h' },
	{ key: '4h', label: '4H', candleKey: '4h' },
	{ key: '1d', label: '1D', candleKey: '1d' },
	{ key: '1w', label: '1W', candleKey: '1w' }
];

export const PRO_CHART_THEME = {
	background: '#ffffff',
	panel: '#ffffff',
	panelElevated: '#f8fafc',
	panelSoft: '#f3f6fb',
	border: 'rgba(100, 116, 139, 0.22)',
	borderStrong: 'rgba(71, 85, 105, 0.36)',
	grid: 'rgba(148, 163, 184, 0.22)',
	gridSoft: 'rgba(203, 213, 225, 0.52)',
	text: '#111827',
	textMuted: '#475569',
	textDim: '#64748b',
	bullish: '#089981',
	bullishSoft: 'rgba(8, 153, 129, 0.16)',
	bearish: '#f23645',
	bearishSoft: 'rgba(242, 54, 69, 0.16)',
	neutral: '#64748b',
	accent: '#2563eb',
	ma7: '#d97706',
	ma25: '#c026d3',
	ma99: '#7c3aed',
	volumeMaFast: '#0284c7',
	volumeMaSlow: '#db2777'
} as const;

export const PRO_CHART_SERIES = {
	maPeriods: [7, 25, 99],
	volumeMaPeriods: [9, 20],
	minHeight: 520,
	mobileMinHeight: 420
} as const;
