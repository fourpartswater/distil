<template>
	<div class="create-solutions-form">

		<b-modal
			v-model="showJoinSuccess"
			class="join-preview-modal"
			@shown="onSuccessModalShwon"
			cancel-disabled
			hide-header
			hide-footer>
			<join-datasets-preview
				:preview-table-data="previewTableData"
				:dataset-a="datasetA"
				:dataset-b="datasetB"
				:joined-column="joinedColumn"
				@success="onJoinCommitSuccess"
				@failure="onJoinCommitFailure"
				@close="showJoinSuccess = !showJoinSuccess;">
			</join-datasets-preview>
		</b-modal>

		<error-modal
			:show="showJoinFailure"
			title="Join Failed"
			@close="showJoinFailure = !showJoinFailure">
		</error-modal>

		<div v-if="columnTypesDoNotMatch" class="row justify-content-center mt-3 mb-3 warning-text">
			<i class="fa fa-exclamation-triangle warning-icon mr-2"></i>
			<span v-html="joinWarning"></span>
		</div>

		<div class="row justify-content-center">
			<b-button class="join-button" :variant="joinVariant" @click="previewJoin" :disabled="disableJoin">
				<div class="row justify-content-center">
					<i class="fa fa-check-circle fa-2x mr-2"></i>
					<b>Join Datasets</b>
				</div>
			</b-button>
		</div>

		<div class="join-progress">
			<b-progress v-if="isPending"
				:value="percentComplete"
				variant="outline-secondary"
				striped
				:animated="true"></b-progress>
		</div>
	</div>
</template>

<script lang="ts">

import _ from 'lodash';
import Vue from 'vue';
import localStorage from 'store';
import JoinDatasetsPreview from '../components/JoinDatasetsPreview.vue';
import ErrorModal from '../components/ErrorModal.vue';
import { createRouteEntry } from '../util/routes';
import { Dictionary } from '../util/dict';
import { getters as routeGetters } from '../store/route/module';
import { Dataset, TableData, TableColumn, TableRow } from '../store/dataset/index';
import { getters as datasetGetters, actions as datasetActions } from '../store/dataset/module';
import { getTableDataItems, getTableDataFields } from '../util/data';
import { SELECT_TARGET_ROUTE } from '../store/route';
import { isJoinable } from '../util/types';

export default Vue.extend({
	name: 'join-datasets-form',

	components: {
		JoinDatasetsPreview,
		ErrorModal
	},

	props: {
		datasetA: String as () => string,
		datasetB: String as () => string,
		datasetAColumn: Object as () => TableColumn,
		datasetBColumn: Object as () => TableColumn,
		joinAccuracy: Number as () => number,
	},

	data() {
		return {
			pending: false,
			showJoin: false,
			showJoinSuccess: false,
			showJoinFailure: false,
			joinErrorMessage: null,
			previewTableData: null
		};
	},

	computed: {
		datasets(): Dataset[] {
			return datasetGetters.getDatasets(this.$store);
		},
		columnsSelected(): boolean {
			return !!this.datasetAColumn && !!this.datasetBColumn;
		},
		columnTypesDoNotMatch(): boolean {
			return this.datasetAColumn && this.datasetBColumn && !isJoinable(this.datasetAColumn.type, this.datasetBColumn.type);
		},
		isPending(): boolean {
			return this.pending;
		},
		joinWarning(): string {
			if (this.columnTypesDoNotMatch) {
				return `Unable to join column <b>${this.datasetAColumn.key}</b> of type <b>${this.datasetAColumn.type}</b> with <b>${this.datasetBColumn.key}</b> of type <b>${this.datasetBColumn.type}</b>`;
			}
		},
		disableJoin(): boolean {
			return this.isPending || !this.columnsSelected || this.columnTypesDoNotMatch;
		},
		joinVariant(): string {
			return !this.disableJoin ? 'success' : 'outline-secondary';
		},
		percentComplete(): number {
			return 100;
		},
		joinDataPreviewItems(): TableRow[] {
			return getTableDataItems(this.previewTableData);
		},
		joinDataPreviewFields(): Dictionary<TableColumn> {
			return getTableDataFields(this.previewTableData);
		},
		joinDataPreviewNumRows(): number {
			return this.previewTableData ? this.previewTableData.numRows : 0;
		},
		joinDataPreviewHasData(): boolean {
			return !!this.previewTableData;
		},
		joinedColumn(): string {
			const a =  this.datasetAColumn ? this.datasetAColumn.key : '';
			const b =  this.datasetBColumn ? this.datasetBColumn.key : '';
			return  `${a}-${b}`;
		}
	},

	methods: {

		addRecentDataset(dataset: string) {
			const datasets = localStorage.get('recent-datasets') || [];
			if (datasets.indexOf(dataset) === -1) {
				datasets.unshift(dataset);
				localStorage.set('recent-datasets', datasets);
			}
		},
		previewJoin() {
			// flag as pending
			this.pending = true;

			const a = _.find(this.datasets, d => {
				return d.id === this.datasetA;
			});

			const b = _.find(this.datasets, d => {
				return d.id === this.datasetB;
			});

			// dispatch action that triggers request send to server
			datasetActions.joinDatasetsPreview(this.$store, {
				datasetA: a,
				datasetB: b,
				datasetAColumn: this.datasetAColumn.key,
				datasetBColumn: this.datasetBColumn.key,
				joinAccuracy: this.joinAccuracy
			}).then(tableData => {
				this.pending = false;
				this.showJoinSuccess = true;
				this.previewTableData = tableData;
			}).catch(err => {
				// display error modal
				this.pending = false;
				this.showJoinFailure = true;
				this.previewTableData = null;
			});
		},
		onJoinCommitSuccess(datasetID: string) {
			const entry = createRouteEntry(SELECT_TARGET_ROUTE, {
				dataset: datasetID
			});
			this.$router.push(entry);
			this.addRecentDataset(datasetID);
		},
		onJoinCommitFailure() {
			this.showJoinFailure = true;
		},
		onSuccessModalShwon() {
			// trigger window resize event to notify modal content dimension has changed
			// (fixed-header-table component will listen to this event to resize itself)
			window.dispatchEvent(new Event('resize'));
		},
	}
});
</script>

<style>
.join-button {
	margin: 0 8px;
	width: 35%;
	line-height: 32px !important;
}

.join-preview-modal .modal-dialog {
	position: relative;
	max-width: 80% !important;
	max-height: 90%;
}

.join-modal-button {
	margin: 0 8px;
	width: 25% !important;
	line-height: 32px !important;
}

.join-progress {
	margin: 6px 10%;
}

.check-message-container {
	display: flex;
	justify-content: flex-start;
	flex-direction: row;
	align-items: center;
}

.warning-icon {
	color: #ee0701;
}

.warning-text {
	line-height: 16px;
	font-size: 16px;
}
</style>
