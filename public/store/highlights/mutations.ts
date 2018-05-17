import _ from 'lodash';
import Vue from 'vue';
import { HighlightState } from './index';
import { VariableSummary } from '../dataset/index';
import { isPredicted, isCorrectness } from '../../util/data';
import { Dictionary } from '../../util/dict';

export const mutations = {

	updateHighlightSamples(state: HighlightState, samples: Dictionary<string[]>) {
		state.highlightValues.samples = samples;
	},

	updateHighlightSummaries(state: HighlightState, summary: VariableSummary) {
		if (!summary) {
			return;
		}
		const index = _.findIndex(state.highlightValues.summaries, s => s.name === summary.name);
		if (index !== -1) {
			Vue.set(state.highlightValues.summaries, index, summary);
			return;
		}
		state.highlightValues.summaries.push(summary);
	},

	updatePredictedHighlightSummaries(state: HighlightState, summary: VariableSummary) {
		if (!summary) {
			return;
		}
		const index = _.findIndex(state.highlightValues.summaries, s => s.solutionId === summary.solutionId && isPredicted(s.name));
		if (index !== -1) {
			Vue.set(state.highlightValues.summaries, index, summary);
			return;
		}
		state.highlightValues.summaries.push(summary);
	},

	updateCorrectnessHighlightSummaries(state: HighlightState, summary: VariableSummary) {
		if (!summary) {
			return;
		}
		const index = _.findIndex(state.highlightValues.summaries, s => s.solutionId === summary.solutionId && isCorrectness(s.name));
		if (index !== -1) {
			Vue.set(state.highlightValues.summaries, index, summary);
			return;
		}
		state.highlightValues.summaries.push(summary);
	},

	clearHighlightSummaries(state: HighlightState) {
		state.highlightValues.summaries = [];
	}
}