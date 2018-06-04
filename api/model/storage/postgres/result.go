package postgres

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
	"github.com/unchartedsoftware/distil/api/model"
	log "github.com/unchartedsoftware/plog"
)

const (
	predictedSuffix   = "_predicted"
	errorSuffix       = "_error"
	correctnessSuffix = "_correctness"
	targetSuffix      = "_target"
)

func (s *Storage) getResultTable(dataset string) string {
	return fmt.Sprintf("%s_result", dataset)
}

func (s *Storage) getResultTargetName(dataset string, resultURI string) (string, error) {
	// Assume only a single target / result. Read the target name from the
	// database table.
	sql := fmt.Sprintf("SELECT target FROM %s WHERE result_id = $1 LIMIT 1;", dataset)

	rows, err := s.client.Query(sql, resultURI)
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("Unable to get target variable name from results for result URI `%s`", resultURI))
	}
	defer rows.Close()

	if rows.Next() {
		var targetName string
		err = rows.Scan(&targetName)
		if err != nil {
			return "", errors.Wrap(err, fmt.Sprintf("Unable to get target variable name for result URI `%s`", resultURI))
		}

		return targetName, nil
	}

	return "", errors.Errorf("Target feature for result URI `%s` not found", resultURI)
}

func (s *Storage) getResultTargetVariable(dataset string, targetName string) (*model.Variable, error) {
	variable, err := s.metadata.FetchVariable(dataset, targetName)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to get target variable information")
	}

	return variable, nil
}

// PersistResult stores the solution result to Postgres.
func (s *Storage) PersistResult(dataset string, resultURI string) error {
	// Read the results file.
	file, err := os.Open(resultURI)
	if err != nil {
		return errors.Wrap(err, "unable open solution result file")
	}
	csvReader := csv.NewReader(bufio.NewReader(file))
	csvReader.TrimLeadingSpace = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return errors.Wrap(err, "unable load solution result as csv")
	}
	if len(records) <= 0 || len(records[0]) <= 0 {
		return errors.Wrap(err, "solution csv empty")
	}

	// currently only support a single result column.
	if len(records[0]) > 2 {
		log.Warnf("Result contains %s columns, expected 2.  Additional columns will be ignored.", len(records[0]))
	}

	// Header row will have the target.
	targetName := records[0][1]

	// Translate from display name to storage name.
	variables, err := s.metadata.FetchVariables(dataset, false)
	if err != nil {
		return errors.Wrap(err, "unable load solution result as csv")
	}

	for _, v := range variables {
		if v.DisplayVariable == targetName {
			targetName = v.OriginalVariable
		}
	}

	// store all results to the storage
	for i := 1; i < len(records); i++ {
		// Each data row is index, target.
		err = nil

		// handle the parsed result/error - should be an int some TA2 systems return floats
		if err != nil {
			return errors.Wrap(err, "failed csv value parsing")
		}
		parsedVal, err := strconv.ParseInt(records[i][0], 10, 64)
		if err != nil {
			parsedValFloat, err := strconv.ParseFloat(records[i][0], 64)
			if err != nil {
				return errors.Wrap(err, "failed csv index parsing")
			}
			parsedVal = int64(parsedValFloat)
		}

		// store the result to the storage
		err = s.executeInsertResultStatement(dataset, resultURI, parsedVal, targetName, records[i][1])
		if err != nil {
			return errors.Wrap(err, "failed to insert result in database")
		}
	}

	return nil
}

func (s *Storage) executeInsertResultStatement(dataset string, resultID string, index int64, target string, value string) error {
	statement := fmt.Sprintf("INSERT INTO %s (result_id, index, target, value) VALUES ($1, $2, $3, $4);", s.getResultTable(dataset))

	_, err := s.client.Exec(statement, resultID, index, target, value)

	return err
}

func (s *Storage) parseVariableValue(value string, variable *model.Variable) (interface{}, error) {
	// Integer types can be returned as floats.
	switch variable.Type {
	case model.IntegerType:
		return strconv.ParseFloat(value, 64)
	case model.FloatType:
		return strconv.ParseFloat(value, 64)
	case model.LongitudeType:
		return strconv.ParseFloat(value, 64)
	case model.LatitudeType:
		return strconv.ParseFloat(value, 64)
	case model.CategoricalType:
		fallthrough
	case model.TextType:
		fallthrough
	case model.DateTimeType:
		fallthrough
	case model.OrdinalType:
		return value, nil
	case model.BoolType:
		return strconv.ParseBool(value)
	default:
		return value, nil
	}
}

func (s *Storage) parseFilteredResults(dataset string, numRows int, rows *pgx.Rows, target *model.Variable) (*model.FilteredData, error) {
	result := &model.FilteredData{
		Name:    dataset,
		NumRows: numRows,
		Values:  make([][]interface{}, 0),
	}

	// Parse the columns.
	if rows != nil {
		fields := rows.FieldDescriptions()
		columns := make([]string, len(fields))
		types := make([]string, len(fields))
		for i := 0; i < len(fields); i++ {
			columns[i] = fields[i].Name
			types[i] = fields[i].DataTypeName
		}

		// Result type provided by DB needs to be overridden with defined target type.
		types[0] = target.Type

		// Parse the row data.
		for rows.Next() {
			columnValues, err := rows.Values()
			if err != nil {
				return nil, errors.Wrap(err, "Unable to extract fields from query result")
			}
			result.Values = append(result.Values, columnValues)
			result.Columns = columns
			result.Types = types
		}
	} else {
		result.Columns = make([]string, 0)
		result.Types = make([]string, 0)
	}

	return result, nil
}

func (s *Storage) parseResults(dataset string, numRows int, rows *pgx.Rows, variable *model.Variable) (*model.FilteredData, error) {
	// Scan the rows. Each row has only the value as a string.
	values := [][]interface{}{}
	for rows.Next() {
		var value string
		err := rows.Scan(&value)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to parse result row")
		}

		val, err := s.parseVariableValue(value, variable)
		if err != nil {
			return nil, errors.Wrap(err, "failed string value parsing")
		}
		values = append(values, []interface{}{val})
	}
	// Build the filtered data.
	return &model.FilteredData{
		Name:    dataset,
		NumRows: numRows,
		Columns: []string{variable.Name},
		Types:   []string{variable.Type},
		Values:  values,
	}, nil
}

func appendAndClause(expression string, andClause string) string {
	if expression == "" {
		return andClause
	}
	if andClause == "" {
		return andClause
	}
	return fmt.Sprintf("%s AND %s", expression, andClause)
}

func isCorrectnessCategory(categoryName string) bool {
	return strings.EqualFold(CorrectCategory, categoryName) || strings.EqualFold(categoryName, IncorrectCategory)
}

func addCorrectnessFilterToWhere(target *model.Variable, correctnessCategory string, wheres string) string {
	// filter for result correctness which is based on well know category values
	categoryWhere := ""
	op := ""
	if strings.EqualFold(correctnessCategory, CorrectCategory) {
		op = "="
	} else if strings.EqualFold(correctnessCategory, IncorrectCategory) {
		op = "!="
	}
	categoryWhere = fmt.Sprintf("predicted.value %s data.\"%s\"", op, target.Name)
	return appendAndClause(wheres, categoryWhere)
}

func addPredictedFilterToWhere(dataset string, predictedFilter *model.Filter, target *model.Variable, wheres string, params []interface{}) (string, []interface{}, error) {
	// Handle the predicted column, which is accessed as `value` in the result query
	where := ""
	switch predictedFilter.Type {
	case model.NumericalFilter:
		// numerical range-based filter
		where = fmt.Sprintf("cast(value AS double precision) >= $%d AND cast(value AS double precision) <= $%d", len(params)+1, len(params)+2)
		params = append(params, *predictedFilter.Min)
		params = append(params, *predictedFilter.Max)

	case model.CategoricalFilter:
		// categorical label based filter, with checks for special correct/incorrect metafilters
		categories := make([]string, 0)
		correctnessCategory := ""
		offset := len(params) + 1

		for i, category := range predictedFilter.Categories {
			if !isCorrectnessCategory(category) {
				categories = append(categories, fmt.Sprintf("$%d", offset+i))
				params = append(params, category)
			} else {
				correctnessCategory = category
			}
		}

		if len(categories) >= 1 {
			where = fmt.Sprintf("value IN (%s)", strings.Join(categories, ", "))
		}

		if correctnessCategory != "" {
			where = addCorrectnessFilterToWhere(target, correctnessCategory, where)
		}

	case model.RowFilter:
		// row index based filter
		indices := make([]string, 0)
		offset := len(params) + 1
		for i, d3mIndex := range predictedFilter.D3mIndices {
			indices = append(indices, fmt.Sprintf("$%d", offset+i))
			params = append(params, d3mIndex)

		}
		if len(indices) >= 1 {
			where = fmt.Sprintf("value IN (%s)", strings.Join(indices, ", "))
		}

	default:
		return "", nil, errors.Errorf("unexpected type %s for variable %s", predictedFilter.Type, predictedFilter.Name)
	}

	// Append the AND clause
	wheres = appendAndClause(wheres, where)
	return wheres, params, nil
}

func addErrorFilterToWhere(dataset string, targetName string, errorFilter *model.Filter, wheres string, params []interface{}) (string, []interface{}, error) {
	// Add a clause to filter residuals to the existing where
	typedError := getErrorTyped(targetName)
	where := fmt.Sprintf("%s >= $%d AND %s <= $%d", typedError, len(params)+1, typedError, len(params)+2)
	params = append(params, *errorFilter.Min)
	params = append(params, *errorFilter.Max)

	// Append the AND clause
	wheres = appendAndClause(wheres, where)
	return wheres, params, nil
}

// FetchFilteredResults pulls the results from the Postgres database.
func (s *Storage) FetchFilteredResults(dataset string, resultURI string, filterParams *model.FilterParams) (*model.FilteredData, error) {
	datasetResult := s.getResultTable(dataset)
	targetName, err := s.getResultTargetName(datasetResult, resultURI)
	if err != nil {
		return nil, err
	}

	// fetch the variable info to resolve its type - skip the first column since that will be the d3m_index value
	variable, err := s.getResultTargetVariable(dataset, targetName)
	if err != nil {
		return nil, err
	}

	// fetch variable metadata
	variables, err := s.metadata.FetchVariables(dataset, false)
	if err != nil {
		return nil, errors.Wrap(err, "Could not pull variables from ES")
	}

	// break filters out groups for specific handling
	filters := s.splitFilters(filterParams)

	// generate variable list for inclusion in query select
	fields, err := s.buildFilteredResultQueryField(dataset, variables, variable, filterParams.Variables)
	if err != nil {
		return nil, errors.Wrap(err, "Could not build field list")
	}

	// Create the filter portion of the where clause.
	where, params := s.buildFilteredQueryWhere(dataset, filters.genericFilters)

	// Add the predicted filter into the where clause if it was included in the filter set
	if filters.predictedFilter != nil {
		where, params, err = addPredictedFilterToWhere(dataset, filters.predictedFilter, variable, where, params)
		if err != nil {
			return nil, errors.Wrap(err, "Could not add result to where clause")
		}
	}

	// Add the error filter into the where clause if it was included in the filter set
	if filters.errorFilter != nil {
		where, params, err = addErrorFilterToWhere(dataset, targetName, filters.errorFilter, where, params)
		if err != nil {
			return nil, errors.Wrap(err, "Could not add error to where clause")
		}
	}

	// If our results are numerical we need to compute residuals and store them in a column called 'error'
	errorExpr := ""
	errorCol := targetName + errorSuffix
	if model.IsNumerical(variable.Type) {
		errorExpr = fmt.Sprintf("%s as \"%s\",", getErrorTyped(variable.Name), errorCol)
	}

	predictedCol := targetName + predictedSuffix
	targetCol := targetName + targetSuffix

	query := fmt.Sprintf(
		"SELECT value as \"%s\", "+
			"\"%s\" as \"%s\", "+
			"%s "+
			"%s "+
			"FROM %s as predicted inner join %s as data on data.\"%s\" = predicted.index "+
			"WHERE result_id = $%d AND target = $%d",
		predictedCol, targetName, targetCol, errorExpr, fields, datasetResult, dataset,
		model.D3MIndexFieldName, len(params)+1, len(params)+2)

	params = append(params, resultURI)
	params = append(params, targetName)

	if len(where) > 0 {
		query = fmt.Sprintf("%s AND %s", query, where)
	}

	// Do not return the whole result set to the client.
	query = fmt.Sprintf("%s LIMIT %d;", query, filterParams.Size)

	rows, err := s.client.Query(query, params...)
	if err != nil {
		return nil, errors.Wrap(err, "Error querying results")
	}
	defer rows.Close()

	countFilter := map[string]interface{}{
		"result_id": resultURI,
	}
	numRows, err := s.FetchNumRows(datasetResult, countFilter)
	if err != nil {
		return nil, errors.Wrap(err, "Could not pull num rows")
	}

	return s.parseFilteredResults(dataset, numRows, rows, variable)
}

// FetchResults pulls the results from the Postgres database.
func (s *Storage) FetchResults(dataset string, resultURI string) (*model.FilteredData, error) {

	// fetch the variable info to resolve its type - skip the first column since that will be the d3m_index value
	datasetResult := s.getResultTable(dataset)
	targetName, err := s.getResultTargetName(datasetResult, resultURI)
	variable, err := s.getResultTargetVariable(dataset, targetName)
	if err != nil {
		return nil, err
	}

	predictedCol := variable.Name + predictedSuffix
	sql := fmt.Sprintf("SELECT value FROM %s as %s WHERE result_id = $1 AND target = $2;", datasetResult, predictedCol)

	rows, err := s.client.Query(sql, resultURI, targetName)
	if err != nil {
		return nil, errors.Wrap(err, "Error querying results")
	}
	defer rows.Close()

	countFilter := map[string]interface{}{
		"result_id": resultURI,
	}
	numRows, err := s.FetchNumRows(datasetResult, countFilter)
	if err != nil {
		return nil, errors.Wrap(err, "Could not pull num rows")
	}

	return s.parseResults(dataset, numRows, rows, variable)
}

func (s *Storage) getResultMinMaxAggsQuery(variable *model.Variable, resultVariable *model.Variable) string {
	// get min / max agg names
	minAggName := model.MinAggPrefix + resultVariable.Name
	maxAggName := model.MaxAggPrefix + resultVariable.Name

	// Only numeric types should occur.
	fieldTyped := fmt.Sprintf("cast(\"%s\" as double precision)", resultVariable.Name)

	// create aggregations
	queryPart := fmt.Sprintf("MIN(%s) AS \"%s\", MAX(%s) AS \"%s\"", fieldTyped, minAggName, fieldTyped, maxAggName)
	// add aggregations
	return queryPart
}

func (s *Storage) getResultHistogramAggQuery(extrema *model.Extrema, variable *model.Variable, resultVariable *model.Variable) (string, string, string) {
	// compute the bucket interval for the histogram
	interval := extrema.GetBucketInterval()

	// Only numeric types should occur.
	fieldTyped := fmt.Sprintf("cast(\"%s\" as double precision)", resultVariable.Name)

	// get histogram agg name & query string.
	histogramAggName := fmt.Sprintf("\"%s%s\"", model.HistogramAggPrefix, extrema.Name)
	rounded := extrema.GetBucketMinMax()
	bucketQueryString := fmt.Sprintf("width_bucket(%s, %g, %g, %d) - 1",
		fieldTyped, rounded.Min, rounded.Max, extrema.GetBucketCount())
	histogramQueryString := fmt.Sprintf("(%s) * %g + %g", bucketQueryString, interval, rounded.Min)

	return histogramAggName, bucketQueryString, histogramQueryString
}

func (s *Storage) fetchResultsExtrema(resultURI string, dataset string, variable *model.Variable, resultVariable *model.Variable) (*model.Extrema, error) {
	// add min / max aggregation
	aggQuery := s.getResultMinMaxAggsQuery(variable, resultVariable)

	// create a query that does min and max aggregations for each variable
	queryString := fmt.Sprintf("SELECT %s FROM %s WHERE result_id = $1 AND target = $2;", aggQuery, dataset)

	// execute the postgres query
	res, err := s.client.Query(queryString, resultURI, variable.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch extrema for result from postgres")
	}
	defer res.Close()

	return s.parseExtrema(res, variable)
}

// FetchResultsExtremaByURI fetches the results extrema by resultURI.
func (s *Storage) FetchResultsExtremaByURI(dataset string, resultURI string) (*model.Extrema, error) {
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

	field := NewNumericalField(s)
	return field.fetchResultsExtrema(resultURI, datasetResult, targetVariable, resultVariable)
}

// FetchResultsSummary gets the summary data about a target variable from the
// results table.
func (s *Storage) FetchResultsSummary(dataset string, resultURI string, filterParams *model.FilterParams, extrema *model.Extrema) (*model.Histogram, error) {
	datasetResult := s.getResultTable(dataset)
	targetName, err := s.getResultTargetName(datasetResult, resultURI)
	if err != nil {
		return nil, err
	}

	variable, err := s.getResultTargetVariable(dataset, targetName)
	if err != nil {
		return nil, err
	}

	// use the variable type to guide the summary creation.
	var field Field
	var histogram *model.Histogram
	if model.IsNumerical(variable.Type) {
		// fetch numeric histograms
		field = NewNumericalField(s)
	} else if model.IsCategorical(variable.Type) {
		// fetch categorical histograms
		field = NewCategoricalField(s)
	} else {
		return nil, errors.Errorf("variable %s of type %s does not support summary", variable.Name, variable.Type)
	}

	histogram, err = field.FetchResultSummaryData(resultURI, dataset, datasetResult, variable, filterParams, extrema)
	if err != nil {
		return nil, errors.Wrap(err, "unable to fetch result summary")
	}

	// add filter if results
	filter := map[string]interface{}{
		"result_id": resultURI,
	}

	// get number of rows
	numRows, err := s.FetchNumRows(datasetResult, filter)
	if err != nil {
		return nil, err
	}
	histogram.NumRows = numRows

	// add dataset
	histogram.Dataset = dataset

	return histogram, nil

}

func toFloat(value interface{}) (float64, error) {
	switch t := value.(type) {
	case int:
		return float64(t), nil
	case int8:
		return float64(t), nil
	case int16:
		return float64(t), nil
	case int32:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case float32:
		return float64(t), nil
	case float64:
		return float64(t), nil
	default:
		return math.NaN(), errors.Errorf("unhandled type %T for %v in conversion to float64", t, value)
	}
}
