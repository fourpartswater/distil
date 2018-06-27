package postgres

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/unchartedsoftware/distil/api/model"
)

// FetchResidualsExtremaByURI fetches the residual extrema by resultURI.
func (s *Storage) FetchResidualsExtremaByURI(dataset string, resultURI string) (*model.Extrema, error) {
	datasetResult := s.getResultTable(dataset)
	targetName, err := s.getResultTargetName(datasetResult, resultURI)
	if err != nil {
		return nil, err
	}
	targetVariable, err := s.getResultTargetVariable(dataset, targetName)
	if err != nil {
		return nil, err
	}
	resultVariable := &model.Variable{
		Name: "value",
		Type: model.TextType,
	}
	return s.fetchResidualsExtrema(resultURI, dataset, targetVariable, resultVariable)
}

// FetchResidualsSummary fetches a histogram of the residuals associated with a set of numerical predictions.
func (s *Storage) FetchResidualsSummary(dataset string, resultURI string, filterParams *model.FilterParams, extrema *model.Extrema) (*model.Histogram, error) {
	datasetResult := s.getResultTable(dataset)
	targetName, err := s.getResultTargetName(datasetResult, resultURI)
	if err != nil {
		return nil, err
	}

	variable, err := s.getResultTargetVariable(dataset, targetName)
	if err != nil {
		return nil, err
	}

	// Just return a nil in the case where we were asked to return residuals for a non-numeric variable.
	if model.IsNumerical(variable.Type) {
		// fetch numeric histograms
		residuals, err := s.fetchResidualsHistogram(resultURI, dataset, variable, extrema)
		if err != nil {
			return nil, err
		}
		return residuals, nil
	}
	return nil, fmt.Errorf("variable is not numeric")
}

func getErrorTyped(variableName string) string {
	return fmt.Sprintf("(cast(value as double precision) - cast(\"%s\" as double precision))", variableName)
}

func (s *Storage) getResidualsHistogramAggQuery(extrema *model.Extrema, variable *model.Variable, resultVariable *model.Variable) (string, string, string) {
	// compute the bucket interval for the histogram
	interval := extrema.GetBucketInterval()

	// Only numeric types should occur.
	errorTyped := getErrorTyped(variable.Name)

	// get histogram agg name & query string.
	histogramAggName := fmt.Sprintf("\"%s%s\"", model.HistogramAggPrefix, extrema.Name)
	rounded := extrema.GetBucketMinMax()
	bucketQueryString := fmt.Sprintf("width_bucket(%s, %g, %g, %d) - 1",
		errorTyped, rounded.Min, rounded.Max, extrema.GetBucketCount())
	histogramQueryString := fmt.Sprintf("(%s) * %g + %g", bucketQueryString, interval, rounded.Min)

	return histogramAggName, bucketQueryString, histogramQueryString
}

func getResultJoin(dataset string) string {
	// FROM clause to join result and base data on d3mIdex value
	return fmt.Sprintf("%s_result as res inner join %s as data on data.\"%s\" = res.index", dataset, dataset, model.D3MIndexFieldName)
}

func getResidualsMinMaxAggsQuery(variable *model.Variable, resultVariable *model.Variable) string {
	// get min / max agg names
	minAggName := model.MinAggPrefix + resultVariable.Name
	maxAggName := model.MaxAggPrefix + resultVariable.Name

	// Only numeric types should occur.
	errorTyped := getErrorTyped(variable.Name)

	// create aggregations
	queryPart := fmt.Sprintf("MIN(%s) AS \"%s\", MAX(%s) AS \"%s\"", errorTyped, minAggName, errorTyped, maxAggName)

	return queryPart
}

func (s *Storage) fetchResidualsExtrema(resultURI string, dataset string, variable *model.Variable,
	resultVariable *model.Variable) (*model.Extrema, error) {
	// add min / max aggregation
	aggQuery := getResidualsMinMaxAggsQuery(variable, resultVariable)

	// from clause to join result and base data
	fromClause := getResultJoin(dataset)

	// create a query that does min and max aggregations for each variable
	queryString := fmt.Sprintf("SELECT %s FROM %s WHERE result_id = $1 AND target = $2;", aggQuery, fromClause)

	// execute the postgres query
	res, err := s.client.Query(queryString, resultURI, variable.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch extrema for result from postgres")
	}
	defer res.Close()

	return s.parseExtrema(res, variable)
}

func (s *Storage) fetchResidualsHistogram(resultURI string, dataset string, variable *model.Variable, extrema *model.Extrema) (*model.Histogram, error) {
	resultVariable := &model.Variable{
		Name: "value",
		Type: model.TextType,
	}

	// need the extrema to calculate the histogram interval
	var err error
	if extrema == nil {
		extrema, err = s.fetchResidualsExtrema(resultURI, dataset, variable, resultVariable)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch result variable extrema for summary")
		}
	} else {
		extrema.Name = variable.Name
		extrema.Type = variable.Type
	}
	// for each returned aggregation, create a histogram aggregation. Bucket
	// size is derived from the min/max and desired bucket count.
	histogramName, bucketQuery, histogramQuery := s.getResidualsHistogramAggQuery(extrema, variable, resultVariable)

	fromClause := getResultJoin(dataset)

	// Create the complete query string.
	query := fmt.Sprintf(`
		SELECT %s as bucket, CAST(%s as double precision) AS %s, COUNT(*) AS count FROM %s
		WHERE result_id = $1 AND target = $2
		GROUP BY %s ORDER BY %s;`, bucketQuery, histogramQuery, histogramName, fromClause, bucketQuery, histogramName)

	// execute the postgres query
	res, err := s.client.Query(query, resultURI, variable.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch histograms for result variable summaries from postgres")
	}
	defer res.Close()

	field := NewNumericalField(s)

	return field.parseHistogram(variable.Type, res, extrema)
}
