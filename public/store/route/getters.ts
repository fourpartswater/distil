import { Variable, VariableSummary } from '../dataset/index';
import { HighlightRoot, RowSelection } from '../highlights/index';
import { JOINED_VARS_INSTANCE_PAGE, AVAILABLE_TARGET_VARS_INSTANCE_PAGE,
	AVAILABLE_TRAINING_VARS_INSTANCE_PAGE, TRAINING_VARS_INSTANCE_PAGE,
	RESULT_TRAINING_VARS_INSTANCE_PAGE } from '../route/index';
import { decodeFilters, Filter, FilterParams } from '../../util/filters';
import { decodeHighlights } from '../../util/highlights';
import { decodeRowSelection } from '../../util/row';
import { Dictionary } from '../../util/dict';
import { buildLookup } from '../../util/lookup';
import { Route } from 'vue-router';
import _ from 'lodash';

export const getters = {
	getRoute(state: Route): Route {
		return state;
	},

	getRoutePath(state: Route): string {
		return state.path;
	},

	getRouteTerms(state: Route): string {
		return state.query.terms as string;
	},

	getRouteDataset(state: Route): string {
		return state.query.dataset as string;
	},

	getRouteJoinDatasets(state: Route): string[] {
		return state.query.joinDatasets ? (state.query.joinDatasets as string).split(',') : [];
	},

	getRouteJoinDatasetsHash(state: Route): string {
		return state.query.joinDatasets as string;
	},

	getJoinDatasetsVariables(state: Route, getters: any): Variable[] {
		const datasetIDs = getters.getRouteJoinDatasets;
		if (datasetIDs.length !== 2) {
			return [];
		}
		const datasets = getters.getDatasets;
		const datasetA = _.find(datasets, d => {
			return d.id === datasetIDs[0];
		});
		const datasetB = _.find(datasets, d => {
			return d.id === datasetIDs[1];
		});
		let variables = [];
		if (datasetA) {
			variables = variables.concat(datasetA.variables);
		}
		if (datasetB) {
			variables = variables.concat(datasetB.variables);
		}
		return variables;
	},

	getJoinDatasetsVariableSummaries(state: Route, getters: any): VariableSummary[] {
		const variables = getters.getJoinDatasetsVariables;
		const lookup = buildLookup(variables.map(v => v.colName));
		const summaries = getters.getVariableSummaries;
		return summaries.filter(summary => lookup[summary.key.toLowerCase()]);
	},

	getJoinDatasetColumnA(state: Route, getters: any): string {
		return state.query.joinColumnA as string;
	},

	getJoinDatasetColumnB(state: Route, getters: any): string {
		return state.query.joinColumnB as string;
	},

	getJoinAccuracy(state: Route, getters: any): number {
		const accuracy = state.query.joinAccuracy;
		return accuracy ? _.toNumber(accuracy) : 1;
	},

	getDecodedJoinDatasetsFilterParams(state: Route, getters: any): Dictionary<FilterParams> {
		const datasetIDs = getters.getRouteJoinDatasets;
		if (datasetIDs.length !== 2) {
			return {};
		}
		const datasets = getters.getDatasets;
		const res = {};

		// build filter params for each dataset
		datasetIDs.forEach(datasetID => {
			const dataset = _.find(datasets, d => {
				return d.id === datasetID;
			});
			if (dataset) {
				const filters = getters.getDecodedFilters;

				// only include filters for this dataset
				const lookup = buildLookup(dataset.variables.map(v => v.colName));
				const filtersForDataset = filters.filter(f => {
					return lookup[f.key];
				});

				const filterParams = _.cloneDeep({
					filters: filtersForDataset,
					variables: dataset.variables.map(v => v.colName)
				});
				res[datasetID] = filterParams;
			}
		});
		return res;
	},

	getRouteTrainingVariables(state: Route): string {
		return state.query.training ? state.query.training as string : null;
	},

	getRouteJoinDatasetsVarsParge(state: Route): number {
		const pageVar = JOINED_VARS_INSTANCE_PAGE;
		return state.query[pageVar] ? _.toNumber(state.query[pageVar]) : 1;
	},

	getRouteAvailableTargetVarsPage(state: Route): number {
		const pageVar = AVAILABLE_TARGET_VARS_INSTANCE_PAGE;
		return state.query[pageVar] ? _.toNumber(state.query[pageVar]) : 1;
	},

	getRouteAvailableTrainingVarsPage(state: Route): number {
		const pageVar = AVAILABLE_TRAINING_VARS_INSTANCE_PAGE;
		return state.query[pageVar] ? _.toNumber(state.query[pageVar]) : 1;
	},

	getRouteTrainingVarsPage(state: Route): number {
		const pageVar = TRAINING_VARS_INSTANCE_PAGE;
		return state.query[pageVar] ? _.toNumber(state.query[pageVar]) : 1;
	},

	getRouteResultTrainingVarsPage(state: Route): number {
		const pageVar = RESULT_TRAINING_VARS_INSTANCE_PAGE;
		return state.query[pageVar] ? _.toNumber(state.query[pageVar]) : 1;
	},

	getRouteTargetVariable(state: Route): string {
		return state.query.target ? state.query.target as string : null;
	},

	getRouteSolutionId(state: Route): string {
		return state.query.solutionId ? state.query.solutionId as string : null;
	},

	getRouteResultId(state: Route): string {
		return state.query.resultId ? state.query.resultId  as string : null;
	},

	getRouteFilters(state: Route): string {
		return state.query.filters ? state.query.filters  as string : null;
	},

	getRouteHighlightRoot(state: Route): string {
		return state.query.highlights ? state.query.highlights  as string : null;
	},

	getRouteRowSelection(state: Route): string {
		return state.query.row ? state.query.row as string : null;
	},

	getRouteResultFilters(state: Route): string {
		return state.query.results ? state.query.results as string : null;
	},

	getRouteResidualThresholdMin(state: Route): string {
		return state.query.residualThresholdMin as string;
	},

	getRouteResidualThresholdMax(state: Route): string {
		return state.query.residualThresholdMax as string;
	},

	getDecodedTrainingVariableNames(state: Route, getters: any): string[] {
		const training = getters.getRouteTrainingVariables;
		return training ? training.split(',') : [];
	},

	getDecodedFilters(state: Route, getters: any): Filter[] {
		return decodeFilters(state.query.filters as string);
	},

	getDecodedFilterParams(state: Route, getters: any): FilterParams {
		const filters = getters.getDecodedFilters;
		const filterParams = _.cloneDeep({
			filters: filters,
			variables: []
		});
		// add training vars
		const training = getters.getDecodedTrainingVariableNames;
		filterParams.variables = filterParams.variables.concat(training);
		// add target vars
		const target = getters.getRouteTargetVariable as string;
		if (target) {
			filterParams.variables.push(target);
		}
		return filterParams;
	},

	getDecodedHighlightRoot(state: Route): HighlightRoot {
		return decodeHighlights(state.query.highlights as string);
	},

	getDecodedRowSelection(state: Route): RowSelection {
		return decodeRowSelection(state.query.row as string);
	},

	getTrainingVariables(state: Route, getters: any): Variable[] {
		const training = getters.getDecodedTrainingVariableNames;
		const lookup = buildLookup(training);
		const variables = getters.getVariables;
		return variables.filter(variable => lookup[variable.colName.toLowerCase()]);
	},

	getTrainingVariableSummaries(state: Route, getters: any): VariableSummary[] {
		const training = getters.getDecodedTrainingVariableNames;
		const lookup = buildLookup(training);
		const summaries = getters.getVariableSummaries;
		return summaries.filter(summary => lookup[summary.key.toLowerCase()]);
	},

	getTargetVariable(state: Route, getters: any): Variable {
		const target = getters.getRouteTargetVariable;
		if (target) {
			const variables = getters.getVariables;
			const found = variables.filter(summary => target.toLowerCase() === summary.colName.toLowerCase());
			if (found) {
				return found[0];
			}
		}
		return null;
	},

	getTargetVariableSummaries(state: Route, getters: any): VariableSummary[] {
		const target = getters.getRouteTargetVariable;
		if (target) {
			const summaries = getters.getVariableSummaries;
			return summaries.filter(summary => target.toLowerCase() === summary.key.toLowerCase());
		}
		return [];
	},

	getAvailableVariables(state: Route, getters: any): Variable[] {
		const training = getters.getDecodedTrainingVariableNames;
		const target = getters.getRouteTargetVariable;
		const lookup = buildLookup(training.concat([ target ]));
		const variables = getters.getVariables;
		return variables.filter(variable => !lookup[variable.colName.toLowerCase()]);
	},

	getAvailableVariableSummaries(state: Route, getters: any): VariableSummary[] {
		const training = getters.getDecodedTrainingVariableNames;
		const target = getters.getRouteTargetVariable;
		const lookup = buildLookup(training.concat([ target ]));
		const summaries = getters.getVariableSummaries;
		return summaries.filter(summary => !lookup[summary.key.toLowerCase()]);
	},

	getActiveSolutionIndex(state: Route, getters: any): number {
		const solutionId = getters.getRouteSolutionId;
		const solutions = getters.getSolutions;
		return _.findIndex(solutions, (solution: any) => {
			return solution.solutionId === solutionId;
		});
	},

	getGeoCenter(state: Route, getters: any): number[] {
		const geo = state.query.geo as string;
		if (!geo) {
			return null;
		}
		const split = geo.split(',');
		return [
			_.toNumber(split[0]),
			_.toNumber(split[1])
		];
	},

	getGeoZoom(state: Route, getters: any): number {
		const geo = state.query.geo as string;
		if (!geo) {
			return null;
		}
		const split = geo.split(',');
		return _.toNumber(split[2]);
	}
};
