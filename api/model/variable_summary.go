package model

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"
)

const (
	// MinAggPrefix is the prefix used for min aggregations.
	MinAggPrefix = "min_"
	// MaxAggPrefix is the prefix used for max aggregations.
	MaxAggPrefix = "max_"
	// TermsAggPrefix is the prefix used for terms aggregations.
	TermsAggPrefix = "terms_"
	// HistogramAggPrefix is the prefix used for histogram aggregations.
	HistogramAggPrefix = "histogram_"
	// VariableValueField is the field which stores the variable value.
	VariableValueField = "value"
	// VariableTypeField is the field which stores the variable's schema type value.
	VariableTypeField = "schemaType"
	// NumBuckets is the number of buckets to use for histograms
	NumBuckets = 50
)

// Extrema represents the extrema for a single variable.
type Extrema struct {
	Name string  `json:"-"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

// Bucket represents a single histogram bucket.
type Bucket struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// Histogram represents a single variable histogram.
type Histogram struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Extrema *Extrema  `json:"extrema,omitempty"`
	Buckets []*Bucket `json:"buckets"`
}

func getNumericalVariables(variables []*Variable) []*Variable {
	var result []*Variable
	for _, variable := range variables {
		if IsNumerical(variable.Type) {
			result = append(result, variable)
		}
	}
	return result
}

func getCategoricalVariables(variables []*Variable) []*Variable {
	var result []*Variable
	for _, variable := range variables {
		if IsCategorical(variable.Type) {
			result = append(result, variable)
		}
	}
	return result
}

func parseExtrema(res *elastic.SearchResult, variable *Variable) (*Extrema, error) {
	// get min / max agg names
	minAggName := MinAggPrefix + variable.Name
	maxAggName := MaxAggPrefix + variable.Name
	// check min agg
	minAgg, ok := res.Aggregations.Min(minAggName)
	if !ok {
		return nil, errors.Errorf("no %s aggregation found", minAggName)
	}
	// check max agg
	maxAgg, ok := res.Aggregations.Max(maxAggName)
	if !ok {
		return nil, errors.Errorf("no %s aggregation found", maxAggName)
	}
	// check values exist
	if minAgg.Value == nil || maxAgg.Value == nil {
		return nil, errors.Errorf("aggregation values found")
	}
	// assign attributes
	return &Extrema{
		Name: variable.Name,
		Min:  *minAgg.Value,
		Max:  *maxAgg.Value,
	}, nil
}

func parseExtremas(res *elastic.SearchResult, variables []*Variable) ([]*Extrema, error) {
	var extremas []*Extrema
	for _, variable := range variables {
		// parse extrema
		extrema, err := parseExtrema(res, variable)
		if err != nil {
			continue
		}
		// append extrema
		extremas = append(extremas, extrema)
	}
	return extremas, nil
}

func appendMinMaxAggs(search *elastic.SearchService, variable *Variable) *elastic.SearchService {
	// get field name
	field := variable.Name + "." + VariableValueField
	// get min / max agg names
	minAggName := MinAggPrefix + variable.Name
	maxAggName := MaxAggPrefix + variable.Name
	// create aggregations
	minAgg := elastic.NewMinAggregation().Field(field)
	maxAgg := elastic.NewMaxAggregation().Field(field)
	// add aggregations
	return search.
		Aggregation(minAggName, minAgg).
		Aggregation(maxAggName, maxAgg)
}

func fetchExtrema(client *elastic.Client, dataset string, variable *Variable) (*Extrema, error) {
	// create a query that does min and max aggregations for each variable
	search := client.Search().
		Index(dataset).
		Size(0)
	// add min / max aggregation
	appendMinMaxAggs(search, variable)
	// execute the search
	res, err := search.Do(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute min/max aggregation query for summary generation")
	}
	return parseExtrema(res, variable)
}

func fetchExtremas(client *elastic.Client, dataset string, variables []*Variable) ([]*Extrema, error) {
	// create a query that does min and max aggregations for each variable
	search := client.Search().
		Index(dataset).
		Size(0)
	// for each variable, create a min / max aggregation
	for _, variable := range variables {
		// add min / max aggregation
		appendMinMaxAggs(search, variable)
	}
	// execute the search
	res, err := search.Do(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute extrema search")
	}
	return parseExtremas(res, variables)
}

func parseNumericHistogram(res *elastic.SearchResult, extrema *Extrema) (*Histogram, error) {
	// get histogram agg name
	histogramAggName := HistogramAggPrefix + extrema.Name
	// get histogram agg
	agg, ok := res.Aggregations.Histogram(histogramAggName)
	if !ok {
		return nil, errors.Errorf("no %s aggregation found", histogramAggName)
	}
	// get histogram buckets
	var buckets []*Bucket
	for _, bucket := range agg.Buckets {
		buckets = append(buckets, &Bucket{
			Key:   strconv.Itoa(int(bucket.Key)),
			Count: bucket.DocCount,
		})
	}
	// assign histogram attributes
	return &Histogram{
		Name:    extrema.Name,
		Type:    "numerical",
		Extrema: extrema,
		Buckets: buckets,
	}, nil
}

func parseNumericHistograms(res *elastic.SearchResult, extremas []*Extrema) ([]*Histogram, error) {
	var histograms []*Histogram
	for _, extrema := range extremas {
		// parse histogram
		histogram, err := parseNumericHistogram(res, extrema)
		if err != nil {
			return nil, err
		}
		// append histogram
		histograms = append(histograms, histogram)
	}
	return histograms, nil
}

func appendHistogramAgg(search *elastic.SearchService, extrema *Extrema) *elastic.SearchService {
	// compute the bucket interval for the histogram
	// TODO: We should handle discreet vs continuous data differently here.  For discrete, we should have
	// a minimum bucket size of 1, whereas continuous can select a size to exactly match the bucket count.
	interval := (extrema.Max - extrema.Min) / NumBuckets

	// get histogram agg name
	histogramAggName := HistogramAggPrefix + extrema.Name
	// create histogram agg
	histogramAgg := elastic.NewHistogramAggregation().
		Field(extrema.Name + "." + VariableValueField).
		Interval(interval)
	// add histogram agg
	return search.Aggregation(histogramAggName, histogramAgg)
}

func fetchNumericalHistogram(client *elastic.Client, dataset string, variable *Variable) (*Histogram, error) {
	// need the extrema to calculate the histogram interval
	extrema, err := fetchExtrema(client, dataset, variable)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch variable extrema for summary")
	}
	// for each returned aggregation, create a histogram aggregation. Bucket
	// size is derived from the min/max and desired bucket count.
	search := client.Search().
		Index(dataset).
		Size(0)
	// add histogram agg
	appendHistogramAgg(search, extrema)
	// execute the search
	res, err := search.Do(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch histograms for variables summaries")
	}
	return parseNumericHistogram(res, extrema)
}

func fetchNumericalHistograms(client *elastic.Client, dataset string, variables []*Variable) ([]*Histogram, error) {
	// need the extrema of each var to calculate the histrogram interval
	extremas, err := fetchExtremas(client, dataset, variables)
	if err != nil {
		return nil, err
	}
	// for each returned aggregation, create a histogram aggregation. Bucket
	// size is derived from the min/max and desired bucket count.
	search := client.Search().
		Index(dataset).
		Size(0)
	// for each extrema, create a histogram aggregation
	for _, extrema := range extremas {
		// add histogram agg
		appendHistogramAgg(search, extrema)
	}
	// execute the search
	res, err := search.Do(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch histograms for variables summaries")
	}
	return parseNumericHistograms(res, extremas)
}

func parseCategoricalHistogram(res *elastic.SearchResult, variable *Variable) (*Histogram, error) {
	// get terms agg name
	termsAggName := TermsAggPrefix + variable.Name
	// check terms agg
	terms, ok := res.Aggregations.Terms(termsAggName)
	if !ok {
		return nil, errors.Errorf("no %s aggregation found", termsAggName)
	}
	// get histogram buckets
	var buckets []*Bucket
	for _, bucket := range terms.Buckets {
		// check value exist
		buckets = append(buckets, &Bucket{
			Key:   bucket.KeyNumber.String(),
			Count: bucket.DocCount,
		})
	}
	// assign histogram attributes
	return &Histogram{
		Name:    variable.Name,
		Type:    "categorical",
		Buckets: buckets,
	}, nil
}

func parseCategoricalHistograms(res *elastic.SearchResult, variables []*Variable) ([]*Histogram, error) {
	var histograms []*Histogram
	for _, variable := range variables {
		// parse histogram
		histogram, err := parseCategoricalHistogram(res, variable)
		if err != nil {
			return nil, err
		}
		// append histogram
		histograms = append(histograms, histogram)
	}
	return histograms, nil
}

func appendTermsAgg(search *elastic.SearchService, variable *Variable) *elastic.SearchService {
	// get field name
	field := variable.Name + "." + VariableValueField
	// get terms agg name
	termsAggName := TermsAggPrefix + variable.Name
	// create aggregation
	termsAgg := elastic.NewTermsAggregation().Field(field)
	// add aggregations
	return search.Aggregation(termsAggName, termsAgg)
}

func fetchCategoricalHistogram(client *elastic.Client, dataset string, variable *Variable) (*Histogram, error) {
	// create a query that does min and max aggregations for each variable
	search := client.Search().
		Index(dataset).
		Size(0)
	// add terms aggregation
	appendTermsAgg(search, variable)
	// execute the search
	res, err := search.Do(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute terms aggregation query for summary")
	}
	return parseCategoricalHistogram(res, variable)
}

func fetchCategoricalHistograms(client *elastic.Client, dataset string, variables []*Variable) ([]*Histogram, error) {
	// create a query that does min and max aggregations for each variable
	search := client.Search().
		Index(dataset).
		Size(0)
	// for each variable, create a min / max aggregation
	for _, variable := range variables {
		// add terms aggregation
		appendTermsAgg(search, variable)
	}
	// execute the search
	res, err := search.Do(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute terms aggregation query for summary")
	}
	return parseCategoricalHistograms(res, variables)
}

// FetchSummary returns the summary for the provided index, dataset, and
// variable.
func FetchSummary(client *elastic.Client, index string, dataset string, varName string) (*Histogram, error) {
	// need list of variables to request aggregation against.
	variable, err := FetchVariable(client, index, dataset, varName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch variables for summary")
	}
	if IsNumerical(variable.Type) {
		// fetch numeric histograms
		numeric, err := fetchNumericalHistogram(client, dataset, variable)
		if err != nil {
			return nil, err
		}
		return numeric, nil
	}
	if IsCategorical(variable.Type) {
		// fetch categorical histograms
		categorical, err := fetchCategoricalHistogram(client, dataset, variable)
		if err != nil {
			return nil, err
		}
		return categorical, nil
	}
	return nil, errors.Errorf("variable %s of type %s does not support summary", variable.Name, variable.Type)
}

// FetchSummaries returns summaries for all variables in the provided index and
// dataset
func FetchSummaries(client *elastic.Client, index string, dataset string) ([]*Histogram, error) {
	// need list of variables to request aggregation against.
	variables, err := FetchVariables(client, index, dataset)
	if err != nil {
		return nil, err
	}
	// fetch numeric histograms
	numerical, err := fetchNumericalHistograms(client, dataset, getNumericalVariables(variables))
	if err != nil {
		return nil, err
	}
	// fetch categorical histograms
	categorical, err := fetchCategoricalHistograms(client, dataset, getCategoricalVariables(variables))
	if err != nil {
		return nil, err
	}
	// merge
	return append(numerical, categorical...), nil
}