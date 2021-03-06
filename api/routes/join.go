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
	"net/http"

	"goji.io/pat"

	"github.com/pkg/errors"
	"github.com/uncharted-distil/distil-ingest/metadata"
	api "github.com/uncharted-distil/distil/api/model"
	"github.com/uncharted-distil/distil/api/task"
	"github.com/uncharted-distil/distil/api/util/json"
)

// JoinHandler generates a route handler that joins two datasets using caller supplied
// columns.  The joined data is returned to the caller, but is NOT added to storage.
func JoinHandler(metaCtor api.MetadataStorageCtor) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// get dataset name
		datasetIDLeft := pat.Param(r, "dataset-left")
		sourceLeft := pat.Param(r, "source-left")
		columnLeft := pat.Param(r, "column-left")
		datasetIDRight := pat.Param(r, "dataset-right")
		columnRight := pat.Param(r, "column-right")
		sourceRight := pat.Param(r, "source-right")

		// get storage client
		storage, err := metaCtor()
		if err != nil {
			handleError(w, err)
			return
		}

		// fetch vars for each dataset
		datasetLeft, err := storage.FetchDataset(datasetIDLeft, true, true)
		if err != nil {
			handleError(w, err)
			return
		}

		datasetRight, err := storage.FetchDataset(datasetIDRight, true, true)
		if err != nil {
			handleError(w, err)
		}

		leftJoin := &task.JoinSpec{
			Column:        columnLeft,
			DatasetID:     datasetLeft.ID,
			DatasetFolder: datasetLeft.Folder,
			DatasetSource: metadata.DatasetSource(sourceLeft),
		}

		rightJoin := &task.JoinSpec{
			Column:        columnRight,
			DatasetID:     datasetRight.ID,
			DatasetFolder: datasetRight.Folder,
			DatasetSource: metadata.DatasetSource(sourceRight),
		}

		// run joining pipeline
		data, err := task.Join(leftJoin, rightJoin, datasetLeft.Variables, datasetRight.Variables)
		if err != nil {
			handleError(w, err)
			return
		}

		// marshal output into JSON
		bytes, err := json.Marshal(data)
		if err != nil {
			handleError(w, errors.Wrap(err, "unable marshal filtered data result into JSON"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)
	}
}
