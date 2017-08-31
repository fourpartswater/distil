/**
 * Structures and functions to support conditional display of pipeline create parameters.
 * The displayName is intended to be a label that is visible
 */

import _ from 'lodash';

// Gets a task object based on a variable type.
export function getTask(varType) {
	const lowerType = _.toLower(varType);
	return _.get(variableType, [lowerType, 'task'], {});
}

// Gets the display names for the metrics from a given task.
export function getMetricDisplayNames(task) {
	return _.map(_.get(task, 'metrics', []), s => s.displayName);
}

// Gets the schema names for the output types for a given task.
export function getOutputSchemaNames(task) {
	return _.map(_.get(task, 'outputs', []), s => s.schemaName);
}

// Gets the schema name for a metric given its display name.
export function getMetricSchemaName(displayName) {
	for(const m of metrics) {
		const result = _.find(m, s => s.displayName === displayName);
		if (!_.isEmpty(result)) {
			return result.schemaName;
		}
	}
	return undefined;
}

// Gets the display name for a metric given its schema name.
export function getMetricDisplayName(schemaName) {
	const lowerName = _.toLower(schemaName);
	for(const m of metrics) {
		const result = _.find(m, s => s.schemaName === lowerName);
		if (!_.isEmpty(result)) {
			return result.displayName;
		}
	}
	return undefined;
}

// metrics used in classification tasks
const classificationMetrics = {
	accuracy: {
		displayName: 'Accuracy',
		schemaName: 'accuracy'
	},
	f1: {
		displayName: 'F1',
		schemaName: 'f1',
	},
	f1Micro: {
		displayName: 'F1 Micro',
		schemaName: 'f1_micro'
	},
	f1Macro: {
		displayName: 'F1 Macro',
		schemaName: 'f1_macro'
	},
	rocAuc: {
		displayName: 'ROC-AUC',
		schemaName: 'roc_auc'
	},
	rocAucMicro: {
		displayName: 'ROC-AUC Micro',
		schemaName: 'roc_auc_micro'
	},
	rocAucMacro: {
		displayName: 'ROC-AUC Macro',
		schemaName: 'roc_auc_macro'
	},
	jaccardSimilarityScore: {
		displayName: 'Jaccard Similarity',
		schemaName: 'jaccard_similarity_score'
	},
	normalizedMutualInformation: {
		displayName: 'Normalized Mutual Information',
		schemaName: 'normalized_mutual_information'
	}
};

// metrics used in regression tasks
const regressionMetrics = {
	rootMeanSquaredError: {
		displayName: 'Root Mean Squared Error',
		schemaName: 'root_mean_squared_error'
	},
	rootMeanSquaredErrorAvg: {
		displayName: 'Root Mean Squared Error Avg',
		schemaName: 'root_mean_squared_error_avg'
	},
	meanAbsoluteErr: {
		displayName: 'Mean Absolute Error',
		schemaName: 'mean_abs_err'
	},
	rSquared: {
		displayName: 'R Squared',
		schemaName: 'r_squared'
	}
};

const metrics = [classificationMetrics, regressionMetrics];

// output types used in classification tasks
const classificationOutputs = {
	classLabel: {
		displayName: 'Label',
		schemaName: 'class_label'
	}
	// multilabel: {
	// 	displayName: 'Multi Label',
	// 	schemaName: 'multilabel'
	// }
};

// output types used in regression tasks
const regressionOutputs = {
	regressionValue: {
		displayName: 'Regression Value',
		schemaName: 'regression_value'
	}
	// probability: {
	// 	displayName: 'Probability',
	// 	schemaName: 'probability'
	// },
	// generalScore: {
	// 	displayName: 'General Score',
	// 	schemaName: 'general_score'
	// }
};

// classification task info
const classification = {
	displayName: 'Classification',
	schemaName: 'classification',
	metrics: classificationMetrics,
	outputs: classificationOutputs
};

// regression task info
const regression = {
	displayName: 'Regression',
	schemaName: 'regression',
	metrics: regressionMetrics,
	outputs: regressionOutputs
};



// variable type to task mappings
const variableType = {
	float: {
		task: regression
	},
	integer: {
		task: regression
	},
	categorical: {
		task: classification
	},
	ordinal: {
		task: classification
	},
	boolean: {
		task: classification
	},
	text: {
		task: classification
	}
};