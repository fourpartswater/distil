package task

import (
	"bytes"
	"encoding/csv"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/unchartedsoftware/distil-ingest/metadata"

	"github.com/unchartedsoftware/distil/api/util"
)

// ClusterPrimitive will cluster the dataset fields using a primitive.
func ClusterPrimitive(index string, dataset string, config *IngestTaskConfig) error {
	// create required folders for outputPath
	createContainingDirs(config.getTmpAbsolutePath(config.ClusteringOutputDataRelative))
	createContainingDirs(config.getTmpAbsolutePath(config.ClusteringOutputSchemaRelative))

	// load metadata from original schema
	meta, err := metadata.LoadMetadataFromOriginalSchema(config.getAbsolutePath(config.SchemaPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load original schema file")
	}
	mainDR := meta.GetMainDataResource()

	// add feature variables
	features, err := getClusterVariables(meta, "_cluster_")
	if err != nil {
		return errors.Wrap(err, "unable to get cluster variables")
	}

	d3mIndexField := getD3MIndexField(mainDR)

	// open the input file
	dataPath := path.Join(config.ContainerDataPath, mainDR.ResPath)
	lines, err := readCSVFile(dataPath, config.HasHeader)
	if err != nil {
		return errors.Wrap(err, "error reading raw data")
	}

	// add the cluster data to the raw data
	for _, f := range features {
		mainDR.Variables = append(mainDR.Variables, f.Variable)

		// header already removed, lines does not have a header
		lines, err = appendFeature(dataset, d3mIndexField, false, f, lines)
		if err != nil {
			return errors.Wrap(err, "error appending clustered data")
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
		return errors.Wrap(err, "error storing clustered header")
	}

	for _, line := range lines {
		err = writer.Write(line)
		if err != nil {
			return errors.Wrap(err, "error storing clustered output")
		}
	}

	// output the data with the new feature
	writer.Flush()

	err = util.WriteFileWithDirs(config.getTmpAbsolutePath(config.ClusteringOutputDataRelative), output.Bytes(), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "error writing clustered output")
	}

	mainDR.ResPath = config.ClusteringOutputDataRelative

	// write the new schema to file
	err = meta.WriteSchema(config.getTmpAbsolutePath(config.ClusteringOutputSchemaRelative))
	if err != nil {
		return errors.Wrap(err, "unable to store cluster schema")
	}

	return nil
}
