<template>
	<div class='variable-facets row'>
		<div class="col-12 flex-column d-flex h-100">
			<div v-if="enableSearch" class="row align-items-center facet-filters">
				<div class="col-12 flex-column d-flex">
					<b-form-input size="sm" v-model="filter" placeholder="Search" />
				</div>
			</div>
			<div v-if="enableTitle" class="row align-items-center">
				<div class="col-12 flex-column d-flex">
					<p><b>Select Feature to Predict</b> Select from potential features of interest below. Each feature tile shown summarizes count of records by value.</p>
				</div>
			</div>
			<div class="pl-1 pr-1">
				<!-- injectable slot -->
				<slot></slot>
			</div>
			<div class="row flex-1">
				<facets class="col-12 flex-column d-flex variable-facets-container h-100"
					:groups="paginatedGroups"
					:highlights="highlights"
					:row-selection="rowSelection"
					:html="html"
					:sort="importanceDesc"
					:enable-type-change="enableTypeChange"
					:enable-highlighting="enableHighlighting"
					:ignore-highlights="ignoreHighlights"
					:instanceName="instanceName"
					@numerical-click="onNumericalClick"
					@categorical-click="onCategoricalClick"
					@range-change="onRangeChange"
					@facet-click="onFacetClick">
				</facets>
			</div>
			<div v-if="numRows > rowsPerPage" class="row align-items-center variable-page-nav">
				<div class="col-12 flex-column">
					<b-pagination size="sm" align="center" :total-rows="numRows" :per-page="rowsPerPage" v-model="currentPage" class="mb-0"/>
				</div>
			</div>
		</div>
	</div>
</template>

<script lang="ts">

import _ from 'lodash';
import Facets from '../components/Facets.vue';
import { overlayRouteEntry, getRouteFacetPage } from '../util/routes';
import { Dictionary } from '../util/dict';
import { sortGroupsByImportance, filterVariablesByPage, getVariableImportance } from '../util/data';
import { Variable } from '../store/dataset/index';
import { Highlight, RowSelection } from '../store/highlights/index';
import { getters as datasetGetters } from '../store/dataset/module';
import { getters as routeGetters } from '../store/route/module';
import { ROUTE_PAGE_SUFFIX } from '../store/route/index';
import { Group } from '../util/facets';
import { updateHighlightRoot, getHighlights, clearHighlightRoot } from '../util/highlights';
import Vue from 'vue';

export default Vue.extend({
	name: 'variable-facets',

	components: {
		Facets
	},

	props: {
		enableSearch: Boolean as () => boolean,
		enableTitle: Boolean as () => boolean,
		enableTypeChange: Boolean as () => boolean,
		enableHighlighting: Boolean as () => boolean,
		ignoreHighlights: Boolean as () => boolean,
		groups: Array as () => Group[],
		subtitle: String as () => string,
		html: [ String as () => string, Object as () => any, Function as () => Function ],
		instanceName: { type: String as () => string, default: 'variableFacets' },
		rowsPerPage: { type: Number as () => number, default: 10 }
	},

	data() {
		return {
			filter: ''
		};
	},

	computed: {

		currentPage: {
			set(page: number) {
				const entry = overlayRouteEntry(this.$route, {
					[this.routePageKey()]: page
				});
				this.$router.push(entry);
			},
			get(): number {
				return getRouteFacetPage(this.routePageKey(), this.$route);
			}
		},

		variables(): Variable[] {
			return datasetGetters.getVariables(this.$store);
		},

		filteredGroups(): Group[] {
			return this.groups.filter(group => {
				return this.filter === '' || group.key.toLowerCase().includes(this.filter.toLowerCase());
			});
		},

		numRows(): number {
			return this.filteredGroups.length;
		},

		sortedFilteredGroups(): Group[] {
			return sortGroupsByImportance(this.filteredGroups, this.variables);
		},

		paginatedGroups(): Group[] {
			const paginated = filterVariablesByPage(this.currentPage, this.rowsPerPage, this.sortedFilteredGroups);

			// TODO: fix this at the Facets component level
			const cloned = _.cloneDeep(paginated);

			// highlight
			if (this.enableHighlighting && this.highlights.root) {
				cloned.forEach(group => {
					if (group) {
						if (group.key === this.highlights.root.key) {
							group.facets.forEach(facet => {
								facet.filterable = true;
							});
						}
					}
				});
			}

			return cloned;
		},

		highlights(): Highlight {
			return getHighlights();
		},

		rowSelection(): RowSelection {
			return routeGetters.getDecodedRowSelection(this.$store);
		},

		importance(): Dictionary<number> {
			const importance: Dictionary<number> = {};
			this.variables.forEach(variable => {
				importance[variable.colName] = getVariableImportance(variable);
			});
			return importance;
		}
	},

	methods: {
		importanceDesc(a: { key: string }, b: { key: string }): number {
			const importance = this.importance;
			return importance[b.key] - importance[a.key];
		},

		// creates a facet key for the route from the instance-name component arg
		// or uses a default if unset
		routePageKey(): string {
			return `${this.instanceName}${ROUTE_PAGE_SUFFIX}`;
		},

		onRangeChange(context: string, key: string, value: { from: number, to: number }, dataset: string) {
			updateHighlightRoot(this.$router, {
				context: context,
				dataset: dataset,
				key: key,
				value: value
			});
			this.$emit('range-change', key, value);
		},

		onFacetClick(context: string, key: string, value: string, dataset: string) {
			if (this.enableHighlighting) {
				if (key && value) {
					updateHighlightRoot(this.$router, {
						context: context,
						dataset: dataset,
						key: key,
						value: value
					});
				} else {
					clearHighlightRoot(this.$router);
				}
			}
			this.$emit('facet-click', context, key, value);
		},

		onCategoricalClick(context: string, key: string) {
			this.$emit('categorical-click', key);
		},

		onNumericalClick(context: string, key: string, value: { from: number, to: number }, dataset: string) {
			if (this.enableHighlighting) {
				if (!this.highlights.root || this.highlights.root.key !== key) {
					updateHighlightRoot(this.$router, {
						context: this.instanceName,
						dataset: dataset,
						key: key,
						value: value
					});
				}
			}
			this.$emit('numerical-click', key);
		},

		availableVariables(): string[] {
			// NOTE: used externally, not internally by the component

			// filter by search
			const searchFiltered = this.groups.filter(group => {
				return this.filter === '' || group.key.toLowerCase().includes(this.filter.toLowerCase());
			});
			return searchFiltered.map(v => v.key);
		}
	}
});

</script>

<style>
button {
	cursor: pointer;
}
.page-link {
	color: #868e96;
}
.page-item.active .page-link {
	z-index: 2;
	color: #fff;
	background-color: #868e96;
	border-color: #868e96;
}
.variable-facets-container {
	overflow-x: hidden;
	overflow-y: auto;
}
.variable-facets-container .facets-root-container{
	margin: 2px;
}
.variable-facets-container .facets-root-container .facets-group-container{
	background-color: inherit;
}
.variable-facets-container .facets-root-container .facets-group-container .facets-group {
	background: white;
	margin: 2px 2px 4px 2px;
	font-size: 0.867rem;
	color: rgba(0,0,0,0.87);
	box-shadow: 0 1px 2px 0 rgba(0,0,0,0.10);
	transition: box-shadow 0.3s ease-in-out;
}
.variable-facets-container .facets-root-container .facets-group-container .facets-group .group-header {
	padding: 4px 8px 6px 8px;
}
.variable-facets-container .facets-root-container .facets-group-container .facets-group .group-header .type-change-menu {
	float: right;
	margin-top: -4px;
	margin-right: -8px;
}
.facet-filters {
	margin: 0 -10px 4px -10px;
}
.facet-filters span {
	font-size: 0.9rem;
}
.variable-page-nav {
	padding-top: 10px;
}

</style>
