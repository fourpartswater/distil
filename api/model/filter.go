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

package model

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"

	"github.com/uncharted-distil/distil-compute/model"
	"github.com/uncharted-distil/distil/api/util/json"
)

// FilterParams defines the set of numeric range and categorical filters. Variables
// with no range or category filters are also allowed.
type FilterParams struct {
	Size      int             `json:"size"`
	Filters   []*model.Filter `json:"filters"`
	Variables []string        `json:"variables"`
}

// Merge merges another set of filter params into this set, expanding all
// properties.
func (f *FilterParams) Merge(other *FilterParams) {
	// take greater of sizes
	if other.Size > f.Size {
		f.Size = other.Size
	}
	for _, filter := range other.Filters {
		found := false
		for _, currentFilter := range f.Filters {
			if filter.Key == currentFilter.Key &&
				filter.Min == currentFilter.Min &&
				filter.Max == currentFilter.Max &&
				filter.Bounds.MinX == currentFilter.Bounds.MinX &&
				filter.Bounds.MaxX == currentFilter.Bounds.MaxX &&
				filter.Bounds.MinY == currentFilter.Bounds.MinY &&
				filter.Bounds.MaxY == currentFilter.Bounds.MaxY &&
				model.StringSliceEqual(filter.Categories, currentFilter.Categories) {
				found = true
				break
			}
		}
		if !found {
			f.Filters = append(f.Filters, filter)
		}
	}
	for _, variable := range other.Variables {
		found := false
		for _, currentVariable := range f.Variables {
			if variable == currentVariable {
				found = true
				break
			}
		}
		if !found {
			f.Variables = append(f.Variables, variable)
		}
	}
}

// Column represents a column for filtered data.
type Column struct {
	Label string `json:"label"`
	Key   string `json:"key"`
	Type  string `json:"type"`
}

// FilteredData provides the metadata and raw data values that match a supplied
// input filter.
type FilteredData struct {
	NumRows int             `json:"numRows"`
	Columns []Column        `json:"columns"`
	Values  [][]interface{} `json:"values"`
}

// GetFilterVariables builds the filtered list of fields based on the filtering parameters.
func GetFilterVariables(filterVariables []string, variables []*model.Variable) []*model.Variable {
	variableLookup := make(map[string]*model.Variable)
	for _, v := range variables {
		variableLookup[v.Name] = v
	}

	filtered := make([]*model.Variable, 0)
	for _, variable := range filterVariables {
		filtered = append(filtered, variableLookup[variable])
		// check for feature var type
		if model.HasFeatureVar(variableLookup[variable].Type) {
			featureVarName := fmt.Sprintf("%s%s", model.FeatureVarPrefix, variable)
			featureVar, ok := variableLookup[featureVarName]
			if ok {
				filtered = append(filtered, featureVar)
			}
		}
		// check for cluster var type
		if model.HasClusterVar(variableLookup[variable].Type) {
			clusterVarName := fmt.Sprintf("%s%s", model.ClusterVarPrefix, variable)
			clusterVar, ok := variableLookup[clusterVarName]
			if ok {
				filtered = append(filtered, clusterVar)
			}
		}
	}

	return filtered
}

// ParseFilterParamsFromJSON parses filter parameters out of a map[string]interface{}
func ParseFilterParamsFromJSON(params map[string]interface{}) (*FilterParams, error) {
	filterParams := &FilterParams{
		Size: json.IntDefault(params, model.DefaultFilterSize, "size"),
	}

	filters, ok := json.Array(params, "filters")
	if ok {
		for _, filter := range filters {

			// type
			typ, ok := json.String(filter, "type")
			if !ok {
				return nil, errors.Errorf("no `type` provided for filter")
			}

			// mode
			mode, ok := json.String(filter, "mode")
			if !ok {
				return nil, errors.Errorf("no `mode` provided for filter")
			}

			// TODO: update to a switch statement with a default to error

			// numeric
			if typ == model.NumericalFilter {
				key, ok := json.String(filter, "key")
				if !ok {
					return nil, errors.Errorf("no `key` provided for filter")
				}
				min, ok := json.Float(filter, "min")
				if !ok {
					return nil, errors.Errorf("no `min` provided for filter")
				}
				max, ok := json.Float(filter, "max")
				if !ok {
					return nil, errors.Errorf("no `max` provided for filter")
				}
				filterParams.Filters = append(filterParams.Filters, model.NewNumericalFilter(key, mode, min, max))
			}

			// bivariate
			if typ == model.BivariateFilter {
				key, ok := json.String(filter, "key")
				if !ok {
					return nil, errors.Errorf("no `key` provided for filter")
				}
				minX, ok := json.Float(filter, "minX")
				if !ok {
					return nil, errors.Errorf("no `minX` provided for filter")
				}
				maxX, ok := json.Float(filter, "maxX")
				if !ok {
					return nil, errors.Errorf("no `maxX` provided for filter")
				}
				minY, ok := json.Float(filter, "minY")
				if !ok {
					return nil, errors.Errorf("no `minY` provided for filter")
				}
				maxY, ok := json.Float(filter, "maxY")
				if !ok {
					return nil, errors.Errorf("no `maxY` provided for filter")
				}
				filterParams.Filters = append(filterParams.Filters, model.NewBivariateFilter(key, mode, minX, maxX, minY, maxY))
			}

			// categorical
			if typ == model.CategoricalFilter {
				key, ok := json.String(filter, "key")
				if !ok {
					return nil, errors.Errorf("no `key` provided for filter")
				}
				categories, ok := json.StringArray(filter, "categories")
				if !ok {
					return nil, errors.Errorf("no `categories` provided for filter")
				}
				filterParams.Filters = append(filterParams.Filters, model.NewCategoricalFilter(key, mode, categories))
			}

			// feature
			if typ == model.FeatureFilter {
				key, ok := json.String(filter, "key")
				if !ok {
					return nil, errors.Errorf("no `key` provided for filter")
				}
				categories, ok := json.StringArray(filter, "categories")
				if !ok {
					return nil, errors.Errorf("no `categories` provided for filter")
				}
				filterParams.Filters = append(filterParams.Filters, model.NewFeatureFilter(key, mode, categories))
			}

			// text
			if typ == model.TextFilter {
				key, ok := json.String(filter, "key")
				if !ok {
					return nil, errors.Errorf("no `key` provided for filter")
				}
				categories, ok := json.StringArray(filter, "categories")
				if !ok {
					return nil, errors.Errorf("no `categories` provided for filter")
				}
				filterParams.Filters = append(filterParams.Filters, model.NewTextFilter(key, mode, categories))
			}

			// row
			if typ == model.RowFilter {
				indices, ok := json.StringArray(filter, "d3mIndices")
				if !ok {
					return nil, errors.Errorf("no `d3mIndices` provided for filter")
				}
				filterParams.Filters = append(filterParams.Filters, model.NewRowFilter(mode, indices))
			}
		}
	}

	variables, ok := json.StringArray(params, "variables")
	if ok {
		filterParams.Variables = variables
	}

	sort.SliceStable(filterParams.Filters, func(i, j int) bool {
		return filterParams.Filters[i].Key < filterParams.Filters[j].Key
	})

	return filterParams, nil
}
