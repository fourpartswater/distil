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

package task

import (
	"bytes"
	"encoding/csv"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/uncharted-distil/distil-compute/model"
	"github.com/uncharted-distil/distil-ingest/metadata"

	"github.com/uncharted-distil/distil/api/util"
)

const (
	unicornResultFieldName = "pred_class"
	slothResultFieldName   = "0"
)

// Cluster will cluster the dataset fields using a primitive.
func Cluster(datasetSource metadata.DatasetSource, schemaFile string, index string, dataset string, config *IngestTaskConfig) (string, error) {
	outputPath, err := initializeDatasetCopy(schemaFile, dataset, config.ClusteringOutputSchemaRelative, config.ClusteringOutputDataRelative)
	if err != nil {
		return "", errors.Wrap(err, "unable to copy source data folder")
	}

	// load metadata from original schema
	meta, err := metadata.LoadMetadataFromOriginalSchema(schemaFile)
	if err != nil {
		return "", errors.Wrap(err, "unable to load original schema file")
	}
	mainDR := meta.GetMainDataResource()

	// add feature variables
	features, err := getClusterVariables(meta, model.ClusterVarPrefix)
	if err != nil {
		return "", errors.Wrap(err, "unable to get cluster variables")
	}

	d3mIndexField := getD3MIndexField(mainDR)

	// open the input file
	dataPath := path.Join(outputPath.sourceFolder, mainDR.ResPath)
	lines, err := ReadCSVFile(dataPath, config.HasHeader)
	if err != nil {
		return "", errors.Wrap(err, "error reading raw data")
	}

	// add the cluster data to the raw data
	for _, f := range features {
		mainDR.Variables = append(mainDR.Variables, f.Variable)

		// header already removed, lines does not have a header
		lines, err = appendFeature(dataset, d3mIndexField, false, f, lines)
		if err != nil {
			return "", errors.Wrap(err, "error appending clustered data")
		}
	}

	// initialize csv writer
	output := &bytes.Buffer{}
	writer := csv.NewWriter(output)

	// output the header
	header := make([]string, len(mainDR.Variables))
	for _, v := range mainDR.Variables {
		header[v.Index] = v.Name
	}
	err = writer.Write(header)
	if err != nil {
		return "", errors.Wrap(err, "error storing clustered header")
	}

	for _, line := range lines {
		err = writer.Write(line)
		if err != nil {
			return "", errors.Wrap(err, "error storing clustered output")
		}
	}

	// output the data with the new feature
	writer.Flush()

	err = util.WriteFileWithDirs(outputPath.outputData, output.Bytes(), os.ModePerm)
	if err != nil {
		return "", errors.Wrap(err, "error writing clustered output")
	}

	relativePath := getRelativePath(path.Dir(outputPath.outputSchema), outputPath.outputData)
	mainDR.ResPath = relativePath

	// write the new schema to file
	err = metadata.WriteSchema(meta, outputPath.outputSchema)
	if err != nil {
		return "", errors.Wrap(err, "unable to store cluster schema")
	}

	return outputPath.outputSchema, nil
}
