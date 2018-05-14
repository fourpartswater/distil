import axios from 'axios';
import { SolutionInfo, SolutionState, SOLUTION_COMPLETED, SOLUTION_ERRORED } from './index';
import { ActionContext } from 'vuex';
import { DistilState } from '../store';
import { mutations } from './module';
import { getWebSocketConnection } from '../../util/ws';
import { FilterParams } from '../../util/filters';
import { regression } from '../../util/solutions';

const ES_INDEX = 'datasets';
const CREATE_SOLUTIONS = 'CREATE_SOLUTIONS';

interface CreateSolutionRequest {
	dataset: string;
	target: string;
	task: string;
	maxSolutions: number;
	metrics: string[];
	filters: FilterParams;
}

export type AppContext = ActionContext<SolutionState, DistilState>;

function updateCurrentSolutionResults(context: any, req: CreateSolutionRequest, res: SolutionInfo) {

	const currentSolutionId = context.getters.getRouteSolutionId;

	// if current solutionId, pull results
	if (res.solutionId === currentSolutionId) {
		context.dispatch('fetchResultTableData', {
			dataset: req.dataset,
			solutionId: res.solutionId
		});
	}

	// if this is a regression task, pull extrema as a first step
	const isRegression = req.task.toLowerCase() === regression.schemaName.toLowerCase();
	let extremaFetches = [];
	if (isRegression) {
		extremaFetches = [
			context.dispatch('fetchTargetResultExtrema', {
				dataset: req.dataset,
				variable: req.target,
				solutionId: res.solutionId
			}),
			context.dispatch('fetchPredictedExtrema', {
				dataset: req.dataset,
				solutionId: res.solutionId
			})
		]
	}

	Promise.all(extremaFetches).then(() => {
		// if current solutionId, pull result summaries
		if (res.solutionId === currentSolutionId) {
			context.dispatch('fetchTrainingResultSummaries', {
				dataset: req.dataset,
				solutionId: res.solutionId,
				variables: context.getters.getActiveSolutionVariables,
				extrema: context.getters.getPredictedExtrema
			});
		}
		context.dispatch('fetchPredictedSummary', {
			dataset: req.dataset,
			solutionId: res.solutionId,
			extrema: context.getters.getPredictedExtrema
		});
		context.dispatch('fetchResultHighlightValues', {
			dataset: req.dataset,
			highlightRoot: context.getters.getDecodedHighlightRoot,
			extrema: context.getters.getPredictedExtrema,
			solutionId: res.solutionId,
			requestIds: context.getters.getSolutions,
			variables: context.getters.getActiveSolutionVariables
		});
	});

	if (isRegression) {
		context.dispatch('fetchResidualsExtrema', {
			dataset: req.dataset,
			solutionId: res.solutionId
		}).then(() => {
			context.dispatch('fetchResidualsSummary', {
				dataset: req.dataset,
				solutionId: res.solutionId,
				extrema: context.getters.getResidualExtrema
			});
		});
	} else {
		context.dispatch('fetchCorrectnessSummary', {
			dataset: req.dataset,
			solutionId: res.solutionId
		});
	}
}

function updateSolutionResults(context: any, req: CreateSolutionRequest, res: SolutionInfo) {
	const isRegression = req.task.toLowerCase() === regression.schemaName.toLowerCase();
	let extremaFetches = [];
	if (isRegression) {
		extremaFetches = [
			context.dispatch('fetchTargetResultExtrema', {
				dataset: req.dataset,
				variable: req.target,
				solutionId: res.solutionId
			}),
			context.dispatch('fetchPredictedExtrema', {
				dataset: req.dataset,
				solutionId: res.solutionId
			})
		]
	}
	Promise.all(extremaFetches).then(() => {
		context.dispatch('fetchPredictedSummary', {
			dataset: req.dataset,
			solutionId: res.solutionId,
			extrema: context.getters.getPredictedExtrema
		});
	});

	if (isRegression) {
		context.dispatch('fetchResidualsExtrema', {
			dataset: req.dataset,
			solutionId: res.solutionId
		}).then(() => {
			context.dispatch('fetchResidualsSummary', {
				dataset: req.dataset,
				solutionId: res.solutionId,
				extrema: context.getters.getResidualExtrema
			});
		});
	} else {
		context.dispatch('fetchCorrectnessSummary', {
			dataset: req.dataset,
			solutionId: res.solutionId
		});
	}
}

export const actions = {

	fetchSolution(context: AppContext, args: { solutionId?: string }) {
		if (!args.solutionId) {
			console.warn('`solutionId` argument is missing');
			return null;
		}

		return axios.get(`/distil/solutions/null/null/${args.solutionId}`)
			.then(response => {
				if (!response.data.solutions) {
					return;
				}
				const solutions = response.data.solutions;
				solutions.forEach(solution => {
					// update solution
					mutations.updateSolutionRequests(context, {
						name: solution.feature,
						feature: solution.feature,
						filters: solution.filters,
						features: solution.features,
						requestId: solution.requestId,
						dataset: solution.dataset,
						timestamp: solution.timestamp,
						progress: solution.progress,
						solutionId: solution.solutionId,
						resultId: solution.resultId,
						scores: solution.scores
					});
				});
			})
			.catch(error => {
				console.error(error);
			});
	},

	fetchSolutions(context: AppContext, args: { dataset?: string, target?: string, solutionId?: string }) {
		if (!args.dataset) {
			args.dataset = null;
		}
		if (!args.target) {
			args.target = null;
		}
		if (!args.solutionId) {
			args.solutionId = null;
		}

		mutations.clearSolutionRequests(context);

		return axios.get(`/distil/solutions/${args.dataset}/${args.target}/${args.solutionId}`)
			.then(response => {
				if (!response.data.solutions) {
					return;
				}
				const solutions = response.data.solutions;
				solutions.forEach(solution => {
					// update solution
					mutations.updateSolutionRequests(context, {
						name: solution.feature,
						feature: solution.feature,
						filters: solution.filters,
						features: solution.features,
						requestId: solution.requestId,
						dataset: solution.dataset,
						timestamp: solution.timestamp,
						progress: solution.progress,
						solutionId: solution.solutionId,
						resultId: solution.resultId,
						scores: solution.scores
					});
				});
			})
			.catch(error => {
				console.error(error);
			});
	},

	createSolutions(context: any, request: CreateSolutionRequest) {
		return new Promise((resolve, reject) => {

			const conn = getWebSocketConnection();

			let receivedFirstResponse = false;

			const stream = conn.stream(res => {

				if (res.error) {
					console.error(res.error);
					return;
				}

				res.name = request.target;
				res.feature = request.target;

				// NOTE: 'fetchSolution' must be done first to ensure the
				// resultId is present to fetch summary

				// update solution status
				context.dispatch('fetchSolution', {
					dataset: request.dataset,
					target: request.target,
					solutionId: res.solutionId,
				}).then(() => {
					// update summaries
					if (res.progress === SOLUTION_ERRORED ||
						res.progress === SOLUTION_COMPLETED) {

						// if current solutionId, pull results
						if (res.solutionId === context.getters.getRouteSolutionId) {
							// current solutionId is selected
							updateCurrentSolutionResults(context, request, res);
						} else {
							// current solutionId is NOT selected
							updateSolutionResults(context, request, res);
						}

					}
				});

				// resolve promise on first response
				if (!receivedFirstResponse) {
					receivedFirstResponse = true;
					resolve(res);
				}

				// close stream on complete
				if (res.progress === SOLUTION_COMPLETED) {
					stream.close();
					return;
				}

			});

			// send create solutions request
			stream.send({
				type: CREATE_SOLUTIONS,
				index: ES_INDEX,
				dataset: request.dataset,
				target: request.target,
				task: request.task,
				metrics: request.metrics,
				maxSolutions: request.maxSolutions,
				filters: request.filters
			});
		});
	},
}