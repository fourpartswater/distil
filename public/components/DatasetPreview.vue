<template>
	<div class='card card-result'>
		<div class='dataset-header hover card-header'  variant="dark" @click.stop='setActiveDataset()' v-bind:class='{collapsed: !expanded}'>
			<a class='nav-link'><b>Name:</b> {{dataset.name}}</a>
			<a class='nav-link'><b>Features:</b> {{dataset.variables.length}}</a>
			<a class='nav-link'><b>Rows:</b> {{dataset.numRows}}</a>
			<a class='nav-link'><b>Size:</b> {{formatBytes(dataset.numBytes)}}</a>
			<a v-if="allowImport && !importPending && datamartProvenance(dataset.provenance)">
				<b-button class="dataset-preview-button" variant="danger" @click.stop='importDataset()'>
					<div class="row justify-content-center pl-3 pr-3">
						<i class="fa fa-cloud-download mr-2"></i>
						<b>Import</b>
					</div>
				</b-button></a>
			<a class="nav-link import-progress-bar" v-if="importPending">
				<b-progress
					:value="percentComplete"
					variant="outline-secondary"
					striped
					:animated="true"></b-progress>
			</a>
			<a v-if="allowJoin && !datamartProvenance(dataset.provenance)">
				<b-button class="dataset-preview-button" variant="primary" @click.stop='joinDataset()'>
					<div class="row justify-content-center pl-3 pr-3">
						<i class="fa fa-compress mr-2"></i>
						<b>Join</b>
					</div>
				</b-button>
			</a>
		</div>
		<div class='card-body'>
			<div class='row'>
				<div class='col-4'>
					<span><b>Top features:</b></span>
					<ul>
						<li :key="variable.name" v-for='variable in topVariables'>
							{{variable.colDisplayName}}
						</li>
					</ul>
				</div>
				<div class='col-8'>
					<div v-if="dataset.summaryML.length > 0">
						<span><b>May relate to topics such as:</b></span>
						<p class='small-text'>
							{{dataset.summaryML}}
						</p>
					</div>
					<span><b>Summary:</b></span>
					<p class='small-text'>
						{{dataset.summary || 'n/a'}}
					</p>
				</div>
			</div>

			<div v-if='!expanded' class='card-expanded'>
				<b-button class='full-width hover' variant='outline-secondary' v-on:click='toggleExpansion()'>
					More Details...
				</b-button>
			</div>

			<div v-if='expanded' class='card-expanded'>
				<span><b>Full Description:</b></span>
				<p v-html='highlightedDescription()'></p>
				<b-button class='full-width hover' variant='outline-secondary' v-on:click='toggleExpansion()'>
					Less Details...
				</b-button>
			</div>

		</div>
		<error-modal
			:show="showImportFailure"
			title="Import Failed"
			@close="showImportFailure = !showImportFailure">
		</error-modal>
	</div>

</template>

<script lang="ts">

import _ from 'lodash';
import Vue from 'vue';
import ErrorModal from '../components/ErrorModal.vue';
import { createRouteEntry } from '../util/routes';
import { formatBytes } from '../util/bytes';
import { sortVariablesByImportance, isDatamartProvenance } from '../util/data';
import { getters as routeGetters } from '../store/route/module';
import { Dataset, Variable } from '../store/dataset/index';
import { actions as datasetActions } from '../store/dataset/module';
import { SELECT_TARGET_ROUTE } from '../store/route/index';
import localStorage from 'store';

const NUM_TOP_FEATURES = 5;

export default Vue.extend({
	name: 'dataset-preview',

	components: {
		ErrorModal
	},

	props: {
		dataset: Object as () => Dataset,
		allowImport: Boolean as () => boolean,
		allowJoin: Boolean as () => boolean,
	},

	computed: {
		terms(): string {
			return routeGetters.getRouteTerms(this.$store);
		},
		topVariables(): Variable[] {
			return sortVariablesByImportance(this.dataset.variables.slice(0)).slice(0, NUM_TOP_FEATURES);
		},
		percentComplete(): number {
			return 100;
		}
	},

	data() {
		return {
			expanded: false,
			importPending: false,
			showImportFailure: false
		};
	},

	methods: {
		formatBytes(n: number): string {
			return formatBytes(n);
		},
		setActiveDataset() {
			const entry = createRouteEntry(SELECT_TARGET_ROUTE, {
				dataset: this.dataset.id
			});
			this.$router.push(entry);
			this.addRecentDataset(this.dataset.id);
		},
		toggleExpansion() {
			this.expanded = !this.expanded;
		},
		highlightedDescription(): string {
			const terms = this.terms;
			if (_.isEmpty(terms)) {
				return this.dataset.description;
			}
			const split = terms.split(/[ ,]+/); // split on whitespace
			const joined = split.join('|'); // join
			const regex = new RegExp(`(${joined})(?![^<]*>)`, 'gm');
			return this.dataset.description.replace(regex, '<span class="highlight">$1</span>');
		},
		addRecentDataset(dataset: string) {
			const datasets = localStorage.get('recent-datasets') || [];
			if (datasets.indexOf(dataset) === -1) {
				datasets.unshift(dataset);
				localStorage.set('recent-datasets', datasets);
			}
		},
		importDataset() {
			this.importPending = true;
			datasetActions.importDataset(this.$store, {
				datasetID: this.dataset.id,
				terms: this.terms,
				source: 'contrib',
				provenance: this.dataset.provenance
			}).then(() => {
				this.importPending = false;
			}).catch(() => {
				this.showImportFailure = true;
				this.importPending = false;
			});
		},
		joinDataset() {
			this.$emit('join-dataset', this.dataset.id);
		},
		datamartProvenance(provenance: string): boolean {
			return isDatamartProvenance(provenance);
		}

	}
});
</script>

<style>
.highlight {
	background-color: #87CEFA;
}
.dataset-header {
	display: flex;
	padding: 4px 8px;
	color: white;
	justify-content: space-between;
	border: none;
	border-bottom: 1px solid rgba(0, 0, 0, 0.125);
}
.card-result .card-header {
	background-color: #424242;
}
.card-result .card-header:hover {
	color: #fff;
	background-color: #535353;
}
.dataset-preview-button {
	line-height: 14px !important;
}
.dataset-header:hover {
	text-decoration: underline;
}
.full-width {
	width: 100%;
}
.card-expanded {
	padding-top: 15px;
}
.import-progress-bar {
	position: relative;
	width: 128px;
}
.import-progress-bar .progress {
	height: 22px;
}
</style>
