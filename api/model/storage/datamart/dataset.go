package datamart

import (
	"io/ioutil"
	"path"
	"strings"

	"github.com/pkg/errors"
	"github.com/unchartedsoftware/distil-compute/model"
	"github.com/unchartedsoftware/distil-compute/primitive/compute"
	"github.com/unchartedsoftware/distil-ingest/metadata"
	api "github.com/unchartedsoftware/distil/api/model"
)

const (
	// DatasetSuffix is the suffix for the dataset entry when stored in
	// elasticsearch.
	metadataType     = "metadata"
	datasetsListSize = 1000
	provenance       = "datamart"
)

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
	rawSets, err := s.searchFolders(strings.Fields(terms))
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

func (s *Storage) searchFolders(terms []string) ([]*model.Metadata, error) {
	// cycle through each folder
	folders, err := ioutil.ReadDir(s.uri)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read dataset directory")
	}

	matches := make([]*model.Metadata, 0)
	for _, info := range folders {
		if !info.IsDir() {
			return nil, errors.Errorf("'%s' is not a directory but is in the dataset directory", info.Name())
		}
		// load the metadata
		schemaFilename := path.Join(s.uri, info.Name(), compute.D3MDataSchema)
		meta, err := metadata.LoadMetadataFromOriginalSchema(schemaFilename)
		if err != nil {
			return nil, errors.Wrap(err, "unable to read metadata")
		}

		// check if match
		if datasetMatches(meta, terms) {
			matches = append(matches, meta)
		}
	}

	return matches, nil
}

func datasetMatches(meta *model.Metadata, terms []string) bool {
	// search the columns & description
	if matches(meta.Description, terms) || matches(meta.Summary, terms) ||
		matches(meta.Name, terms) {
		return true
	}

	for _, dr := range meta.DataResources {
		for _, f := range dr.Variables {
			if matches(f.Name, terms) {
				return true
			}
		}
	}

	return false
}

func matches(text string, terms []string) bool {
	//TODO: probably want to weigh matches in some way (more terms matched = better?)
	for _, t := range terms {
		if strings.Contains(text, t) {
			return true
		}
	}

	// if no terms provided, assume match
	return len(terms) == 0
}
