package elastic

import (
	"context"
	"sort"

	"github.com/pkg/errors"
	"github.com/unchartedsoftware/distil/api/model"
	"github.com/unchartedsoftware/distil/api/util/json"
	log "github.com/unchartedsoftware/plog"
	"gopkg.in/olivere/elastic.v5"
)

func (s *Storage) parseResults(searchResults *elastic.SearchResult) (*model.FilteredData, error) {
	var data model.FilteredData

	for idx, hit := range searchResults.Hits.Hits {
		// parse hit into JSON
		src, err := json.Unmarshal(*hit.Source)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse data")
		}

		variables, ok := json.Map(src)
		if !ok {
			return nil, errors.Wrap(err, "failed to parse data")
		}

		// On the first time through, parse out name/type info and store that in a header.  We also
		// store the name/type tuples in a map for quick lookup
		if idx == 0 {
			data.Name = hit.Index
			for key, variable := range variables {
				varType, ok := json.String(variable, model.VariableTypeField)
				if !ok {
					return nil, errors.Errorf("failed to extract type info for %s during metadata creation", key)
				}
				data.Metadata = append(data.Metadata, &model.Variable{Name: key, Type: varType})
			}
			// sort to impose consistent ordering
			sort.SliceStable(data.Metadata, func(i, j int) bool {
				return data.Metadata[i].Name < data.Metadata[j].Name
			})
		}

		// Create a temporary metadata -> index map.  Required because the variable data for each hit returned
		//  from ES is an unordered key/value list.
		metadataIndex := make(map[string]int, len(data.Metadata))
		for idx, value := range data.Metadata {
			metadataIndex[value.Name] = idx
		}

		// extract data for all variables
		values := make([]interface{}, len(data.Metadata))
		for key, variable := range variables {
			index := metadataIndex[key]
			result, ok := json.Interface(variable, model.VariableValueField)
			if !ok {
				log.Errorf("%+v", err)
			}
			values[index] = result
		}
		// add the row to the variable data
		data.Values = append(data.Values, values)
	}
	return &data, nil
}

// FetchData creates an ES query to fetch a set of documents.  Applies filters to restrict the
// results to a user selected set of fields, with documents further filtered based on allowed ranges and
// categories.
func (s *Storage) FetchData(dataset string, filterParams *model.FilterParams) (*model.FilteredData, error) {
	// construct an ES query that fetches documents from the dataset with the supplied variable filters applied
	query := elastic.NewBoolQuery()
	var excludes []string
	for _, variable := range filterParams.Ranged {
		query = query.Filter(elastic.NewRangeQuery(variable.Name + ".value").Gte(variable.Min).Lte(variable.Max))
	}
	for _, variable := range filterParams.Categorical {
		// this is imposed by go's language design - []string needs explicit conversion to []interface{} before
		// passing to interface{} ...
		categories := make([]interface{}, len(variable.Categories))
		for i := range variable.Categories {
			categories[i] = variable.Categories[i]
		}
		query = query.Filter(elastic.NewTermsQuery(variable.Name+".value", categories...))
	}
	for _, variableName := range filterParams.None {
		excludes = append(excludes, variableName)
	}

	fetchContext := elastic.NewFetchSourceContext(true).Exclude(excludes...)

	// execute the ES query
	res, err := s.client.Search().
		Query(query).
		Index(dataset).
		Size(filterParams.Size).
		FetchSource(true).
		FetchSourceContext(fetchContext).
		Do(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "elasticsearch filtered data query failed")
	}

	// parse the result
	return s.parseResults(res)
}