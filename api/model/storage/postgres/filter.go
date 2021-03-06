//
//   Copyright © 2019 Uncharted Software Inc.
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package postgres

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx"
	"github.com/pkg/errors"
	"github.com/uncharted-distil/distil-compute/model"
	api "github.com/uncharted-distil/distil/api/model"
)

const (
	// CorrectCategory identifies the correct result meta-category.
	CorrectCategory = "correct"

	// IncorrectCategory identifies the incorrect result meta-category.
	IncorrectCategory = "incorrect"
)

func getVariableByKey(key string, variables []*model.Variable) *model.Variable {
	for _, variable := range variables {
		if variable.Name == key {
			return variable
		}
	}
	return nil
}

func (s *Storage) parseFilteredData(dataset string, variables []*model.Variable, numRows int, rows *pgx.Rows) (*api.FilteredData, error) {
	result := &api.FilteredData{
		NumRows: numRows,
		Values:  make([][]interface{}, 0),
	}

	// Parse the columns.
	if rows != nil {
		fields := rows.FieldDescriptions()
		columns := make([]api.Column, len(fields))
		for i := 0; i < len(fields); i++ {
			key := fields[i].Name

			v := getVariableByKey(key, variables)
			if v == nil {
				return nil, fmt.Errorf("unable to lookup variable for %s", key)
			}
			columns[i] = api.Column{
				Key:   key,
				Label: v.DisplayName,
				Type:  v.Type,
			}
		}
		result.Columns = columns

		// Parse the row data.
		for rows.Next() {
			columnValues, err := rows.Values()
			if err != nil {
				return nil, err
			}
			result.Values = append(result.Values, columnValues)
		}
	} else {
		result.Columns = make([]api.Column, 0)
	}

	return result, nil
}

func (s *Storage) formatFilterKey(key string) string {
	if api.IsResultKey(key) {
		return "result.value"
	}
	return fmt.Sprintf("\"%s\"", key)
}

func (s *Storage) buildIncludeFilter(wheres []string, params []interface{}, filter *model.Filter) ([]string, []interface{}) {

	name := s.formatFilterKey(filter.Key)

	switch filter.Type {
	case model.NumericalFilter:
		// numerical
		// cast to double precision in case of string based representation
		where := fmt.Sprintf("cast(%s as double precision) >= $%d AND cast(%s as double precision) <= $%d", name, len(params)+1, name, len(params)+2)
		wheres = append(wheres, where)
		params = append(params, *filter.Min)
		params = append(params, *filter.Max)

	case model.BivariateFilter:
		// bivariate
		// cast to double precision in case of string based representation
		split := strings.Split(filter.Key, ":")
		where := ""
		if len(split) > 1 {
			xKey := split[0]
			yKey := split[1]
			where = fmt.Sprintf("cast(%s as double precision) >= $%d AND cast(%s as double precision) <= $%d AND cast(%s as double precision) >= $%d AND cast(%s as double precision) <= $%d", xKey, len(params)+1, xKey, len(params)+2, yKey, len(params)+3, yKey, len(params)+4)
		} else {
			// hardcode [lat, lon] format for now
			where = fmt.Sprintf("%s[2] >= $%d AND %s[2] <= $%d AND %s[1] >= $%d AND %s[1] <= $%d", filter.Key, len(params)+1, filter.Key, len(params)+2, filter.Key, len(params)+3, filter.Key, len(params)+4)
		}
		wheres = append(wheres, where)
		params = append(params, filter.Bounds.MinX)
		params = append(params, filter.Bounds.MaxX)
		params = append(params, filter.Bounds.MinY)
		params = append(params, filter.Bounds.MaxY)

	case model.CategoricalFilter:
		// categorical
		categories := make([]string, 0)
		offset := len(params) + 1
		for i, category := range filter.Categories {
			categories = append(categories, fmt.Sprintf("$%d", offset+i))
			params = append(params, category)
		}
		where := fmt.Sprintf("%s IN (%s)", name, strings.Join(categories, ", "))
		wheres = append(wheres, where)
	case model.RowFilter:
		// row
		indices := make([]string, 0)
		offset := len(params) + 1
		for i, d3mIndex := range filter.D3mIndices {
			indices = append(indices, fmt.Sprintf("$%d", offset+i))
			params = append(params, d3mIndex)
		}
		where := fmt.Sprintf("\"%s\" IN (%s)", model.D3MIndexFieldName, strings.Join(indices, ", "))
		wheres = append(wheres, where)
	case model.FeatureFilter, model.TextFilter:
		// feature
		offset := len(params) + 1
		for i, category := range filter.Categories {
			where := fmt.Sprintf("%s ~* (%s)", name, fmt.Sprintf("$%d", offset+i))
			params = append(params, category)
			wheres = append(wheres, where)
		}
	}
	return wheres, params
}

func (s *Storage) buildExcludeFilter(wheres []string, params []interface{}, filter *model.Filter) ([]string, []interface{}) {

	name := s.formatFilterKey(filter.Key)

	switch filter.Type {
	case model.NumericalFilter:
		// numerical
		where := fmt.Sprintf("(%s < $%d OR %s > $%d)", name, len(params)+1, name, len(params)+2)
		wheres = append(wheres, where)
		params = append(params, *filter.Min)
		params = append(params, *filter.Max)

	case model.BivariateFilter:
		// bivariate
		// cast to double precision in case of string based representation
		split := strings.Split(filter.Key, ":")
		where := ""
		if len(split) > 1 {
			xKey := split[0]
			yKey := split[1]
			where = fmt.Sprintf("(%s < $%d OR %s > $%d) OR (%s < $%d OR %s > $%d)", xKey, len(params)+1, xKey, len(params)+2, yKey, len(params)+3, yKey, len(params)+4)
		} else {
			// hardcode [lat, lon] format for now
			where = fmt.Sprintf("(%s[2] < $%d OR %s[2] > $%d) OR (%s[1] < $%d OR %s[1] > $%d)", filter.Key, len(params)+1, filter.Key, len(params)+2, filter.Key, len(params)+3, filter.Key, len(params)+4)
		}
		wheres = append(wheres, where)
		params = append(params, filter.Bounds.MinX)
		params = append(params, filter.Bounds.MaxX)
		params = append(params, filter.Bounds.MinY)
		params = append(params, filter.Bounds.MaxY)

	case model.CategoricalFilter:
		// categorical
		categories := make([]string, 0)
		offset := len(params) + 1
		for i, category := range filter.Categories {
			categories = append(categories, fmt.Sprintf("$%d", offset+i))
			params = append(params, category)
		}
		where := fmt.Sprintf("%s NOT IN (%s)", name, strings.Join(categories, ", "))
		wheres = append(wheres, where)
	case model.RowFilter:
		// row
		indices := make([]string, 0)
		offset := len(params) + 1
		for i, d3mIndex := range filter.D3mIndices {
			indices = append(indices, fmt.Sprintf("$%d", offset+i))
			params = append(params, d3mIndex)
		}
		where := fmt.Sprintf("\"%s\" NOT IN (%s)", model.D3MIndexFieldName, strings.Join(indices, ", "))
		wheres = append(wheres, where)
	case model.FeatureFilter, model.TextFilter:
		// feature
		offset := len(params) + 1
		for i, category := range filter.Categories {
			where := fmt.Sprintf("%s !~* (%s)", name, fmt.Sprintf("$%d", offset+i))
			params = append(params, category)
			wheres = append(wheres, where)
		}
	}
	return wheres, params
}

func (s *Storage) buildFilteredQueryWhere(wheres []string, params []interface{}, filters []*model.Filter) ([]string, []interface{}) {
	for _, filter := range filters {
		switch filter.Mode {
		case model.IncludeFilter:
			wheres, params = s.buildIncludeFilter(wheres, params, filter)
		case model.ExcludeFilter:
			wheres, params = s.buildExcludeFilter(wheres, params, filter)
		}
	}
	return wheres, params
}

func (s *Storage) buildFilteredQueryField(variables []*model.Variable, filterVariables []string) (string, error) {
	fields := make([]string, 0)
	indexIncluded := false
	for _, variable := range api.GetFilterVariables(filterVariables, variables) {
		fields = append(fields, fmt.Sprintf("\"%s\"", variable.Name))
		if variable.Name == model.D3MIndexFieldName {
			indexIncluded = true
		}
	}
	// if the index is not already in the field list, then append it
	if !indexIncluded {
		fields = append(fields, fmt.Sprintf("\"%s\"", model.D3MIndexFieldName))
	}
	return strings.Join(fields, ","), nil
}

func (s *Storage) buildFilteredResultQueryField(variables []*model.Variable, targetVariable *model.Variable, filterVariables []string) (string, error) {
	fields := make([]string, 0)
	for _, variable := range api.GetFilterVariables(filterVariables, variables) {
		if strings.Compare(targetVariable.Name, variable.Name) != 0 {
			fields = append(fields, fmt.Sprintf("\"%s\"", variable.Name))
		}
	}
	fields = append(fields, fmt.Sprintf("\"%s\"", model.D3MIndexFieldName))
	return strings.Join(fields, ","), nil
}

func (s *Storage) buildCorrectnessResultWhere(wheres []string, params []interface{}, storageName string, resultURI string, resultFilter *model.Filter) ([]string, []interface{}, error) {
	// get the target variable name
	storageNameResult := s.getResultTable(storageName)
	targetName, err := s.getResultTargetName(storageNameResult, resultURI)
	if err != nil {
		return nil, nil, err
	}

	// correct/incorrect are well known categories that require the predicted category to be compared
	// to the target category
	op := ""
	for _, category := range resultFilter.Categories {
		if strings.EqualFold(category, CorrectCategory) {
			op = "="
			break
		} else if strings.EqualFold(category, IncorrectCategory) {
			op = "!="
			break
		}
	}
	if op == "" {
		return nil, nil, err
	}
	where := fmt.Sprintf("result.value %s data.\"%s\"", op, targetName)
	wheres = append(wheres, where)
	return wheres, params, nil
}

func (s *Storage) buildErrorResultWhere(wheres []string, params []interface{}, residualFilter *model.Filter) ([]string, []interface{}, error) {
	// Add a clause to filter residuals to the existing where
	nameWithoutSuffix := api.StripKeySuffix(residualFilter.Key)
	typedError := getErrorTyped(nameWithoutSuffix)
	where := fmt.Sprintf("(%s >= $%d AND %s <= $%d)", typedError, len(params)+1, typedError, len(params)+2)
	params = append(params, *residualFilter.Min)
	params = append(params, *residualFilter.Max)

	// Append the AND clause
	wheres = append(wheres, where)
	return wheres, params, nil
}

func (s *Storage) buildPredictedResultWhere(wheres []string, params []interface{}, resultURI string, resultFilter *model.Filter) ([]string, []interface{}, error) {
	// handle the general category case
	wheres, params = s.buildFilteredQueryWhere(wheres, params, []*model.Filter{resultFilter})
	return wheres, params, nil
}

func (s *Storage) buildResultQueryFilters(storageName string, resultURI string, filterParams *api.FilterParams) ([]string, []interface{}, error) {
	// pull filters generated against the result facet out for special handling
	filters := s.splitFilters(filterParams)

	// create the filter for the query
	wheres := make([]string, 0)
	params := make([]interface{}, 0)
	wheres, params = s.buildFilteredQueryWhere(wheres, params, filters.genericFilters)

	// assemble split filters
	var err error
	if filters.predictedFilter != nil {
		wheres, params, err = s.buildPredictedResultWhere(wheres, params, resultURI, filters.predictedFilter)
		if err != nil {
			return nil, nil, err
		}
	} else if filters.correctnessFilter != nil {
		wheres, params, err = s.buildCorrectnessResultWhere(wheres, params, storageName, resultURI, filters.correctnessFilter)
		if err != nil {
			return nil, nil, err
		}
	} else if filters.residualFilter != nil {
		wheres, params, err = s.buildErrorResultWhere(wheres, params, filters.residualFilter)
		if err != nil {
			return nil, nil, err
		}
	}
	return wheres, params, nil
}

type filters struct {
	genericFilters    []*model.Filter
	predictedFilter   *model.Filter
	residualFilter    *model.Filter
	correctnessFilter *model.Filter
}

func (s *Storage) splitFilters(filterParams *api.FilterParams) *filters {
	// Groups filters for handling downstream
	var predictedFilter *model.Filter
	var residualFilter *model.Filter
	var correctnessFilter *model.Filter
	var remaining []*model.Filter
	for _, filter := range filterParams.Filters {
		if api.IsPredictedKey(filter.Key) {
			predictedFilter = filter
		} else if api.IsErrorKey(filter.Key) {
			if filter.Type == model.NumericalFilter {
				residualFilter = filter
			} else if filter.Type == model.CategoricalFilter {
				correctnessFilter = filter
			}
		} else {
			remaining = append(remaining, filter)
		}
	}
	return &filters{
		genericFilters:    remaining,
		predictedFilter:   predictedFilter,
		residualFilter:    residualFilter,
		correctnessFilter: correctnessFilter,
	}
}

// FetchNumRows pulls the number of rows in the table.
func (s *Storage) FetchNumRows(storageName string, filters map[string]interface{}) (int, error) {
	query := fmt.Sprintf("SELECT count(*) FROM %s", storageName)
	params := make([]interface{}, 0)
	if filters != nil && len(filters) > 0 {
		clauses := make([]string, 0)
		for field, value := range filters {
			clauses = append(clauses, fmt.Sprintf("%s = $%d", field, len(clauses)+1))
			params = append(params, value)
		}
		query = fmt.Sprintf("%s WHERE %s", query, strings.Join(clauses, " AND "))
	}
	var numRows int
	err := s.client.QueryRow(query, params...).Scan(&numRows)
	if err != nil {
		return -1, errors.Wrap(err, "postgres row query failed")
	}
	return numRows, nil
}

func (s *Storage) filterIncludesIndex(filterParams *api.FilterParams) bool {
	for _, v := range filterParams.Filters {
		if v.Key == model.D3MIndexFieldName {
			return true
		}
	}

	return false
}

// FetchData creates a postgres query to fetch a set of rows.  Applies filters to restrict the
// results to a user selected set of fields, with rows further filtered based on allowed ranges and
// categories.
func (s *Storage) FetchData(dataset string, storageName string, filterParams *api.FilterParams, invert bool) (*api.FilteredData, error) {
	variables, err := s.metadata.FetchVariables(dataset, true, true)
	if err != nil {
		return nil, errors.Wrap(err, "Could not pull variables from ES")
	}

	numRows, err := s.FetchNumRows(storageName, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Could not pull num rows")
	}

	fields, err := s.buildFilteredQueryField(variables, filterParams.Variables)
	if err != nil {
		return nil, errors.Wrap(err, "Could not build field list")
	}

	// construct a Postgres query that fetches documents from the dataset with the supplied variable filters applied
	query := fmt.Sprintf("SELECT %s FROM %s", fields, storageName)

	wheres := make([]string, 0)
	params := make([]interface{}, 0)
	wheres, params = s.buildFilteredQueryWhere(wheres, params, filterParams.Filters)

	if len(wheres) > 0 {
		if invert {
			query = fmt.Sprintf("%s WHERE NOT(%s)", query, strings.Join(wheres, " AND "))
		} else {
			query = fmt.Sprintf("%s WHERE %s", query, strings.Join(wheres, " AND "))
		}
	} else {
		// if there are not WHERE's and we are inverting, that means we expect
		// no results.
		if invert {
			return &api.FilteredData{
				NumRows: numRows,
				Columns: make([]api.Column, 0),
				Values:  make([][]interface{}, 0),
			}, nil
		}
	}

	// order & limit the filtered data.
	query = fmt.Sprintf("%s ORDER BY \"%s\"", query, model.D3MIndexFieldName)
	if filterParams.Size > 0 {
		query = fmt.Sprintf("%s LIMIT %d", query, filterParams.Size)
	}
	query = query + ";"

	// execute the postgres query
	res, err := s.client.Query(query, params...)
	if err != nil {
		return nil, errors.Wrap(err, "postgres filtered data query failed")
	}
	if res != nil {
		defer res.Close()
	}

	// parse the result
	return s.parseFilteredData(dataset, variables, numRows, res)
}
