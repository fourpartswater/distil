import _ from 'lodash';
import Vue from 'vue';
import { DatasetState, Variable, Dataset, VariableSummary, TableData } from './index';
import { updateSummaries } from '../../util/data';

export const mutations = {

	setDatasets(state: DatasetState, datasets: Dataset[]) {
		state.datasets = datasets;
	},

	setVariables(state: DatasetState, variables: Variable[]) {
		state.variables = variables;
	},

	updateVariableType(state: DatasetState, update) {
		const index = _.findIndex(state.variables, v => {
			return v.colName === update.field;
		});
		state.variables[index].colType = update.type;
	},

	updateVariableSummaries(state: DatasetState, summary: VariableSummary) {
		updateSummaries(summary, state.variableSummaries);
	},

	updateFile(state: DatasetState, args: { url: string, file: any }) {
		Vue.set(state.files, args.url, args.file);
	},

	// sets the current selected data into the store
	setIncludedTableData(state: DatasetState, includedTableData: TableData) {
		state.includedTableData = includedTableData;
	},

	// sets the current excluded data into the store
	setExcludedTableData(state: DatasetState, excludedTableData: TableData) {
		state.excludedTableData = excludedTableData;
	}

}
