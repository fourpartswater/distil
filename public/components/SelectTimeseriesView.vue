<template>

	<div class="select-timeseries-view" @mousemove="mouseMove" @wheel="scroll">
		<div class="timeseries-row-header">
			<div class="timeseries-var-col pad-top"><b>VARIABLES</b></div>
			<div class="timeseries-min-col pad-top"><b>MIN</b></div>
			<div class="timeseries-max-col pad-top"><b>MAX</b></div>
			<div class="timeseries-chart-axis">
				<template v-if="!!timeseriesExtrema">
					<svg ref="svg" class="axis"></svg>
				</template>
			</div>
		</div>
		<div class="timeseries-rows">
			<div v-for="item in items">
				<sparkline-row
					:timeseries-url="item[timeseriesField]"
					:timeseries-extrema="microExtrema"
					:margin="margin"
					:highlight-pixel-x="highlightPixelX">
				</sparkline-row>
			</div>
		</div>
		<div class="vertical-line"></div>
	</div>

</template>

<script lang="ts">

import * as d3 from 'd3';
import _ from 'lodash';
import $ from 'jquery';
import Vue from 'vue';
import SparklineRow from './SparklineRow';
import { Dictionary } from '../util/dict';
import { Filter } from '../util/filters';
import { RowSelection, HighlightRoot } from '../store/highlights/index';
import { TableRow, TableColumn, TimeseriesExtrema } from '../store/dataset/index';
import { getters as routeGetters } from '../store/route/module';
import { getters as datasetGetters } from '../store/dataset/module';
import { updateHighlightRoot } from '../util/highlights';
import { TIMESERIES_TYPE } from '../util/types';

const TICK_SIZE = 8;
const SELECTED_TICK_SIZE = 18;
const MIN_PIXEL_WIDTH = 32;

export default Vue.extend({
	name: 'select-timeseries-view',

	components: {
		SparklineRow
	},

	props: {
		margin: {
			type: Object as () => any,
			default: () => ({
				top: 2,
				right: 16,
				bottom: 2,
				left: 16
			})
		},
		instanceName: String as () => string,
		includedActive: Boolean as () => boolean
	},

	data() {
		return {
			macroScale: null,
			microScale: null,
			microRangeSelection: null,
			selectedMicroMin: null,
			selectedMicroMax: null,
			highlightPixelX: null
		};
	},

	computed: {
		dataset(): string {
			return routeGetters.getRouteDataset(this.$store);
		},

		items(): TableRow[] {
			return this.includedActive ? datasetGetters.getIncludedTableDataItems(this.$store) : datasetGetters.getExcludedTableDataItems(this.$store);
		},

		fields(): Dictionary<TableColumn> {
			return this.includedActive ? datasetGetters.getIncludedTableDataFields(this.$store) : datasetGetters.getExcludedTableDataFields(this.$store);
		},

		timeseriesField(): string {
			const fields = _.map(this.fields, (field, key) => {
					return {
						key: key,
						type: field.type
					};
				})
				.filter(field => field.type === TIMESERIES_TYPE)
				.map(field => field.key);
			return fields[0];
		},

		filters(): Filter[] {
			if (this.includedActive) {
				return this.invertFilters(routeGetters.getDecodedFilters(this.$store));
			}
			return routeGetters.getDecodedFilters(this.$store);
		},

		rowSelection(): RowSelection {
			return routeGetters.getDecodedRowSelection(this.$store);
		},

		timeseriesExtrema(): TimeseriesExtrema {
			const extrema = datasetGetters.getTimeseriesExtrema(this.$store);
			return extrema[this.dataset];
		},

		highlightRoot(): HighlightRoot {
			return routeGetters.getDecodedHighlightRoot(this.$store);
		},

		microExtrema(): TimeseriesExtrema {
			return {
				x: {
					min: this.microMin,
					max: this.microMax
				},
				y: {
					min: this.timeseriesExtrema ? this.timeseriesExtrema.y.min : 0,
					max: this.timeseriesExtrema ? this.timeseriesExtrema.y.max : 1
				}
			};
		},

		width(): number {
			const $svg = this.$refs.svg as any;
			const dims = $svg.getBoundingClientRect();
			return dims.width - this.margin.left - this.margin.right;
		},

		height(): number {
			const $svg = this.$refs.svg as any;
			const dims = $svg.getBoundingClientRect();
			return dims.height - this.margin.top - this.margin.bottom;
		},

		svg(): d3.Selection<SVGElement, {}, HTMLElement, any> {
			const $svg = this.$refs.svg as any;
			return d3.select($svg);
		},

		isTimeseriesViewHighlight(): boolean {
			// ignore any highlights unless they are range highlights
			return this.highlightRoot &&
				this.highlightRoot.key === this.timeseriesField &&
				this.highlightRoot.value.from !== undefined &&
				this.highlightRoot.value.to !== undefined;
		},

		microMin(): number {
			if (this.selectedMicroMin !== null) {
				return this.selectedMicroMin;
			}
			if (this.isTimeseriesViewHighlight) {
				return this.highlightRoot.value.from;
			}
			if (this.timeseriesExtrema) {
				return this.timeseriesExtrema.x.min;
			}
			return 0;
		},

		microMax(): number {
			if (this.selectedMicroMax !== null) {
				return this.selectedMicroMax;
			}
			if (this.isTimeseriesViewHighlight) {
				return this.highlightRoot.value.to;
			}
			if (this.timeseriesExtrema) {
				return this.timeseriesExtrema.x.max;
			}
			return 1;
		}
	},

	methods: {
		invertFilters(filters: Filter[]): Filter[] {
			// TODO: invert filters
			return filters;
		},
		mouseMove(event) {
			const parentOffset = $('.select-timeseries-view').offset();
			const chartBounds = $('.timeseries-chart-axis').offset();
			const chartWidth = $('.timeseries-chart-axis').width();
			const chartScroll = $('.select-timeseries-view').parent().scrollTop();

			const relX = event.pageX - parentOffset.left;

			const chartLeft = chartBounds.left - parentOffset.left;
			if (relX >= chartLeft && relX <= chartLeft + chartWidth) {
				$('.vertical-line').show();
				$('.vertical-line').css({
					left: relX,
					top: chartScroll
				});
				this.highlightPixelX = relX - chartLeft - this.margin.left;
			} else {
				$('.vertical-line').hide();
				this.highlightPixelX = null;
			}
		},
		scroll(event) {
			const chartScroll = $('.select-timeseries-view').parent().scrollTop();
			$('.vertical-line').css('top', chartScroll);
		},
		injectMicroAxis() {

			this.svg.select('.micro-axis').remove();
			this.svg.select('.axis-selection-rect').remove();

			this.microScale = d3.scaleLinear()
				.domain([this.microMin, this.microMax])
				.range([0, this.width]);

			this.svg.append('g')
				.attr('class', 'micro-axis')
				.attr('transform', `translate(${this.margin.left}, ${-this.margin.bottom + this.height - TICK_SIZE * 2})`)
				.call(d3.axisBottom(this.microScale));

			this.svg.append('rect')
				.attr('class', 'axis-selection-rect')
				.attr('x', this.macroScale(this.microMin) + this.margin.left)
				.attr('y', this.margin.top + TICK_SIZE * 2)
				.attr('width', this.macroScale(this.microMax) - this.macroScale(this.microMin))
				.attr('height', SELECTED_TICK_SIZE);

			this.svg.select('.axis-selection').raise();

			this.attachTranslationHandlers();
		},
		injectSVG() {

			if (!this.timeseriesExtrema) {
				return;
			}

			this.clearSVG();

			this.macroScale = d3.scaleLinear()
				.domain([this.timeseriesExtrema.x.min, this.timeseriesExtrema.x.max])
				.range([0, this.width]);

			this.svg.append('g')
				.attr('class', 'macro-axis')
				.attr('transform', `translate(${this.margin.left}, ${this.margin.top + SELECTED_TICK_SIZE + TICK_SIZE * 2})`)
				.call(d3.axisTop(this.macroScale));

			this.microRangeSelection = d3.axisTop(this.macroScale)
				.tickSize(SELECTED_TICK_SIZE)
				.tickValues([
					this.microMin,
					this.microMax
				]);

			this.svg.append('g')
				.attr('class', 'axis-selection')
				.attr('transform', `translate(${this.margin.left}, ${this.margin.top + SELECTED_TICK_SIZE + TICK_SIZE * 2})`)
				.call(this.microRangeSelection);

			this.injectMicroAxis();

			this.attachScalingHandlers();
		},
		repositionMicroMin(xVal: number) {
			const px = this.macroScale(xVal);
			const $lower = this.svg.select('.axis-selection .tick');
			$lower.attr('transform', `translate(${px}, 0)`);
			$lower.select('text').text(xVal.toFixed(2));
		},
		repositionMicroMax(xVal: number) {
			const px = this.macroScale(xVal);
			const $upper = this.svg.select('.axis-selection .tick:last-child');
			$upper.attr('transform', `translate(${px}, 0)`);
			$upper.select('text').text(xVal.toFixed(2));
		},
		repositionMicroRange(xMin: number, xMax: number) {
			const minPx = this.macroScale(xMin);
			const maxPx = this.macroScale(xMax);
			const widthPx = maxPx - minPx;
			const $range = this.svg.select('.axis-selection-rect');
			$range .attr('x', minPx + this.margin.left);
			$range .attr('width', widthPx);
		},
		attachScalingHandlers() {
			const dragstarted = (d, index, elem) => {
				this.highlightPixelX = null;
				$('.vertical-line').hide();
			};

			const dragged = (d, index, elem) => {
				const minX = 0;
				const maxX = this.width;

				const px = _.clamp(d3.event.x, minX, maxX);

				if (index === 0) {
					const maxPx = this.macroScale(this.microMax);
					const clampedPx = Math.min(px, maxPx - MIN_PIXEL_WIDTH);
					this.selectedMicroMin = this.macroScale.invert(clampedPx);
					this.repositionMicroMin(this.selectedMicroMin);
				} else {
					const minPx = this.macroScale(this.microMin);
					const clampedPx = Math.max(px, minPx + MIN_PIXEL_WIDTH);
					this.selectedMicroMax = this.macroScale.invert(clampedPx);
					this.repositionMicroMax(this.selectedMicroMax);
				}

				this.injectMicroAxis();
			};

			const dragended = (d, index, elem) => {
				updateHighlightRoot(this.$router, {
					context: this.instanceName,
					dataset: this.dataset,
					key: this.timeseriesField,
					value: {
						from: this.microMin,
						to: this.microMax
					}
				});
			};

			this.svg.selectAll('.axis-selection .tick')
				.call(d3.drag()
					.on('start', dragstarted)
					.on('drag', dragged)
					.on('end', dragended)
				);
		},
		attachTranslationHandlers() {

			const dragged = (d, index, elem) => {

				if (this.selectedMicroMin === null) {
					this.selectedMicroMin = this.microMin;
				}
				if (this.selectedMicroMax === null) {
					this.selectedMicroMax = this.microMax;
				}

				const maxDelta = this.timeseriesExtrema.x.max - this.selectedMicroMax;
				const minDelta = this.timeseriesExtrema.x.min - this.selectedMicroMin;

				const delta = this.macroScale.invert(d3.event.dx);
				const clampedDelta = _.clamp(delta, minDelta, maxDelta);

				this.selectedMicroMin += clampedDelta;
				this.selectedMicroMax += clampedDelta;

				// update rect
				this.repositionMicroRange(this.selectedMicroMin, this.selectedMicroMax);

				// update ticks
				this.repositionMicroMin(this.selectedMicroMin);
				this.repositionMicroMax(this.selectedMicroMax);

				this.injectMicroAxis();
			};

			const dragended = (d, index, elem) => {
				updateHighlightRoot(this.$router, {
					context: this.instanceName,
					dataset: this.dataset,
					key: this.timeseriesField,
					value: {
						from: this.microMin,
						to: this.microMax
					}
				});
			};

			this.svg.selectAll('.axis-selection-rect')
				.call(d3.drag()
					.on('drag', dragged)
					.on('end', dragended));
		},
		clearSVG() {
			this.svg.selectAll('*').remove();
		}
	},

	watch: {
		timeseriesExtrema: {
			handler() {
				Vue.nextTick(() => {
					this.injectSVG();
				});
			},
			deep: true
		}
	},

	mounted() {
		this.injectSVG();
	}

});
</script>

<style>
svg.axis {
	position: relative;
	max-height: 64px;
	width: 100%;
}
.select-timeseries-view {
	position: relative;
	flex: 1;
	height: inherit;
}
.timeseries-row-header {
	position: relative;
	width: 100%;
	height: 64px;
	line-height: 32px;
	border-bottom: 1px solid #999;
	padding: 0 8px;
	background-color: #fff;
}
.timeseries-rows {
	position: relative;
	height: calc(100% - 64px);
	z-index: 0;
	overflow: scroll;
}
.timeseries-var-col {
	float: left;
	position: relative;
	line-height: 32px;
	height: 32px;
	width: 156px;
}
.timeseries-min-col {
	float: left;
	position: relative;
	line-height: 32px;
	height: 32px;
	width: 48px;
}
.timeseries-max-col {
	float: left;
	position: relative;
	line-height: 32px;
	height: 32px;
	width: 48px;
}
.timeseries-chart-col {
	float: left;
	position: relative;
	line-height: 32px;
	height: 32px;
	width: calc(100% - 276px);
}
.timeseries-chart-axis {
	float: left;
	position: relative;
	line-height: 32px;
	height: 64px;
	width: calc(100% - 276px);
}
.pad-top {
	padding-top: 32px;
}
.vertical-line {
	position: absolute;
	display: none;
	top: 0;
	left: 0;
	width: 1px;
	height: 100%;
	border-left: 1px solid #00c6e1;
	box-shadow: 0px 0px 5px #00c6e1;
	pointer-events: none;
}
.axis-selection {
}

.axis-selection .tick {
	cursor: pointer;
	stroke-width: 3;
}

.axis-selection path.domain {
	visibility: hidden;
}

.axis-selection-rect {
	fill: #00c6e1;
	opacity: 0.25;
	cursor: pointer;
}

</style>
