package datamart

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/unchartedsoftware/distil-compute/model"
	api "github.com/unchartedsoftware/distil/api/model"
	log "github.com/unchartedsoftware/plog"
)

const (
	// DatasetSuffix is the suffix for the dataset entry when stored in
	// elasticsearch.
	metadataType       = "metadata"
	datasetsListSize   = 1000
	provenance         = "datamart"
	searchRESTFunction = "search"
)

type SearchQuery struct {
	Properties *SearchQueryProperties `json:"properties"`
}

type SearchQueryProperties struct {
	Dataset *SearchQueryDatasetProperties `json:"dataset"`
}

type SearchQueryDatasetProperties struct {
	About       string   `json:"about"`
	Description []string `json:"description"`
	Name        []string `json:"name"`
	Keywords    []string `json:"keywords"`
}

// ImportDataset makes the dataset available for ingest and returns
// the URI to use for ingest.
func (s *Storage) ImportDataset(uri string) (string, error) {
	// dataset is already on local file system and accessible for ingest
	return uri, nil
}

// FetchDatasets returns all datasets in the provided index.
func (s *Storage) FetchDatasets(includeIndex bool, includeMeta bool) ([]*api.Dataset, error) {
	// use default string in search to get complete list
	return s.SearchDatasets("", includeIndex, includeMeta)
}

// FetchDataset returns a dataset in the provided index.
func (s *Storage) FetchDataset(datasetName string, includeIndex bool, includeMeta bool) (*api.Dataset, error) {
	return nil, errors.Errorf("Not implemented")
}

// SearchDatasets returns the datasets that match the search criteria in the
// provided index.
func (s *Storage) SearchDatasets(terms string, includeIndex bool, includeMeta bool) ([]*api.Dataset, error) {
	rawSets, err := s.searchREST(terms)
	if err != nil {
		return nil, err
	}

	return s.parseDatasets(rawSets)
}

// SetDataType is not supported by the datamart.
func (s *Storage) SetDataType(dataset string, varName string, varType string) error {
	return errors.Errorf("Not supported")
}

// AddVariable is not supported by the datamart.
func (s *Storage) AddVariable(dataset string, varName string, varType string, varRole string) error {
	return errors.Errorf("Not supported")
}

// DeleteVariable is not supported by the datamart.
func (s *Storage) DeleteVariable(dataset string, varName string) error {
	return errors.Errorf("Not supported")
}

func (s *Storage) parseDatasets(raw []*model.Metadata) ([]*api.Dataset, error) {
	datasets := make([]*api.Dataset, 0)

	for _, meta := range raw {
		// merge all variables into a single set
		// TODO: figure out how we handle multiple data resources!
		vars := make([]*model.Variable, 0)
		for _, dr := range meta.DataResources {
			vars = append(vars, dr.Variables...)
		}
		datasets = append(datasets, &api.Dataset{
			Name:        meta.Name,
			Description: meta.Description,
			Folder:      meta.DatasetFolder,
			Summary:     meta.Summary,
			SummaryML:   meta.SummaryMachine,
			NumRows:     int64(meta.NumRows),
			NumBytes:    int64(meta.NumBytes),
			Variables:   vars,
			Provenance:  provenance,
		})
	}

	return datasets, nil
}

func (s *Storage) searchREST(searchText string) ([]*model.Metadata, error) {
	terms := strings.Fields(searchText)

	// get complete URI for the endpoint
	query := &SearchQuery{
		Properties: &SearchQueryProperties{
			Dataset: &SearchQueryDatasetProperties{
				About:       searchText,
				Name:        terms,
				Description: terms,
				Keywords:    terms,
			},
		},
	}
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, errors.Wrap(err, "unable to marshal datamart query")
	}

	responseRaw, err := s.client.PostJSON(searchRESTFunction, queryJSON)
	if err != nil {
		return nil, errors.Wrap(err, "unable to post datamart search request")
	}
	log.Infof("DATAMART POST %v", responseRaw)

	return nil, nil
}
