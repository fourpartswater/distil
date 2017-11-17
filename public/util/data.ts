import _ from 'lodash';
import { Datasets, Dictionary, FieldInfo, Variable } from '../store/data/index';

// filters datasets by id
export function filterDatasets(ids: string[], datasets: Datasets[]): Datasets[] {
	if (_.isUndefined(ids)) {
		return datasets;
	}
	const idSet = new Set(ids);
	return _.filter(datasets, d => idSet.has(d.name));
}

// fetches datasets from local storage
export function getRecentDatasets(): string[] {
	const datasets = window.localStorage.getItem('recent-datasets');
	return (datasets) ? datasets.split(',') : [];
}

// adds a recent dataset to local storage
export function addRecentDataset(dataset: string) {
	const datasetsStr = window.localStorage.getItem('recent-datasets');
	const datasets = (datasetsStr) ? datasetsStr.split(',') : [];
	datasets.unshift(dataset);
	window.localStorage.setItem('recent-datasets', datasets.join(','));
}

export function isInTrainingSet(col: string, training: Dictionary<Variable>) {
	return (isPredictedIndex(col) ||
		isErrorIndex(col) ||
		isTarget(col) ||
		training[col]);
}

export function removeNonTrainingItems(items: Dictionary<any>[], training: Dictionary<Variable>):  Dictionary<any>[] {
	return _.map(items, item => {
		const row = {};
		_.forIn(item, (val, col) => {
			if (isInTrainingSet(col, training)) {
				row[col] = val;
			}
		});
		return row;
	});
}

export function removeNonTrainingFields(fields: Dictionary<FieldInfo>, training: Dictionary<Variable>): Dictionary<FieldInfo> {
	const res: Dictionary<FieldInfo> = {};
	_.forIn(fields, (val, col) => {
		if (isInTrainingSet(col, training)) {
			res[col] = val;
		}
	});
	return res;
}

export function isPredictedIndex(col: string) {
	return col.endsWith('_res');
}

export function isErrorIndex(col: string) {
	return col === 'error';
}

export function isTarget(col: string) {
	return col === '_target';
}
export function getPredictedIndex(columns: string[]) {
	return _.findIndex(columns, isPredictedIndex);
}

export function getErrorIndex(columns: string[]) {
	return _.findIndex(columns, isErrorIndex);
}
