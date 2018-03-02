package postgres

import (
	"github.com/unchartedsoftware/distil/api/model"
)

// Field defines behaviour for a database field type.
type Field interface {
	FetchSummaryData(dataset string, index string, variable *model.Variable, resultURI string, filterParams *model.FilterParams, inclusive bool, extrema *model.Extrema) (*model.Histogram, error)
	FetchResultSummaryData(resultURI string, dataset string, datasetResult string, variable *model.Variable, extrema *model.Extrema) (*model.Histogram, error)
}
