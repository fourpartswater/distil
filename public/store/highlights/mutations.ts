import _ from 'lodash';
import Vue from 'vue';
import { HighlightState } from './index';
import { VariableSummary } from '../dataset/index';

export const mutations = {

	updateHighlightSummaries(state: HighlightState, summary: VariableSummary) {
		if (!summary) {
			return;
		}
		const index = _.findIndex(state.highlightValues.summaries, s => s.key === summary.key);
		if (index !== -1) {
			Vue.set(state.highlightValues.summaries, index, summary);
			return;
		}
		state.highlightValues.summaries.push(summary);
	},

	clearHighlightSummaries(state: HighlightState) {
		state.highlightValues.summaries = [];
	}
};
