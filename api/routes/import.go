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

package routes

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	"goji.io/pat"

	"github.com/uncharted-distil/distil-ingest/metadata"
	"github.com/uncharted-distil/distil/api/env"
	"github.com/uncharted-distil/distil/api/model"
	"github.com/uncharted-distil/distil/api/model/storage/datamart"
	"github.com/uncharted-distil/distil/api/task"
)

// ImportHandler imports a dataset to the local file system and then ingests it.
func ImportHandler(nyuDatamartMetaCtor model.MetadataStorageCtor, isiDatamartMetaCtor model.MetadataStorageCtor, fileMetaCtor model.MetadataStorageCtor, esMetaCtor model.MetadataStorageCtor, config *task.IngestTaskConfig) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		datasetID := pat.Param(r, "datasetID")
		source := metadata.DatasetSource(pat.Param(r, "source"))
		provenance := pat.Param(r, "provenance")

		// update ingest config to use ingest URI.
		cfg, err := env.LoadConfig()
		if err != nil {
			handleError(w, err)
			return
		}

		meta, err := createMetadataStorageForSource(source, provenance, nyuDatamartMetaCtor, isiDatamartMetaCtor, fileMetaCtor, esMetaCtor)
		if err != nil {
			handleError(w, err)
			return
		}

		// import the dataset to the local filesystem.
		uri := env.ResolvePath(source, datasetID)

		ingestConfig := *config
		ingestConfig.SummaryEnabled = false

		_, err = meta.ImportDataset(datasetID, uri)
		if err != nil {
			handleError(w, err)
			return
		}

		// ingest the imported dataset
		err = task.IngestDataset(source, esMetaCtor, cfg.ESDatasetsIndex, datasetID, &ingestConfig)
		if err != nil {
			handleError(w, err)
			return
		}

		// marshal data and sent the response back
		err = handleJSON(w, map[string]interface{}{"result": "ingested"})
		if err != nil {
			handleError(w, errors.Wrap(err, "unable marshal result histogram into JSON"))
			return
		}
	}
}

func createMetadataStorageForSource(datasetSource metadata.DatasetSource, provenance string,
	nyuDatamartMetaCtor model.MetadataStorageCtor, isiDatamartMetaCtor model.MetadataStorageCtor,
	fileMetaCtor model.MetadataStorageCtor, esMetaCtor model.MetadataStorageCtor) (model.MetadataStorage, error) {
	if datasetSource == metadata.Contrib {
		if provenance == datamart.ProvenanceNYU {
			return nyuDatamartMetaCtor()
		} else if provenance == datamart.ProvenanceISI {
			return isiDatamartMetaCtor()
		}
	}
	if datasetSource == metadata.Seed {
		return esMetaCtor()
	}
	if datasetSource == metadata.Augmented {
		return fileMetaCtor()
	}
	return nil, fmt.Errorf("unrecognized source `%v`", datasetSource)
}
