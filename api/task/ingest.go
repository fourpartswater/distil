package task

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/olivere/elastic.v5"

	"github.com/unchartedsoftware/distil-ingest/conf"
	"github.com/unchartedsoftware/distil-ingest/merge"
	"github.com/unchartedsoftware/distil-ingest/metadata"
	"github.com/unchartedsoftware/distil-ingest/postgres"
	"github.com/unchartedsoftware/distil-ingest/rest"
)

const (
	rankingFilename = "rank-no-missing.csv"
)

// IngestTaskConfig captures the necessary configuration for an data ingest.
type IngestTaskConfig struct {
	ContainerDataPath                string
	DataPathRelative                 string
	DatasetFolderSuffix              string
	HasHeader                        bool
	MergedOutputPathRelative         string
	MergedOutputSchemaPathRelative   string
	SchemaPathRelative               string
	ClassificationRESTEndpoint       string
	ClassificationFunctionName       string
	ClassificationOutputPathRelative string
	RankingRESTEndpoint              string
	RankingFunctionName              string
	RankingOutputPathRelative        string
	DatabasePassword                 string
	DatabaseUser                     string
	Database                         string
	SummaryOutputPathRelative        string
	ESEndpoint                       string
	ESTimeout                        int
	ESDatasetPrefix                  string
}

func (c *IngestTaskConfig) getRootPath(dataset string) string {
	return fmt.Sprintf("%s/%s/%s%s", c.ContainerDataPath, dataset, dataset, c.DatasetFolderSuffix)
}

func (c *IngestTaskConfig) getAbsolutePath(dataset string, relativePath string) string {
	return fmt.Sprintf("%s/%s", c.getRootPath(dataset), relativePath)
}

func (c *IngestTaskConfig) getRawDataPath(dataset string) string {
	return fmt.Sprintf("%s/", c.getRootPath(dataset))
}

// IngestDataset executes the complete ingest process for the specified dataset.
func IngestDataset(index string, dataset string, config *IngestTaskConfig) error {
	err := Merge(index, dataset, config)
	if err != nil {
		return errors.Wrap(err, "unable to merge all data into a single file")
	}

	err = Classify(index, dataset, config)
	if err != nil {
		return errors.Wrap(err, "unable to classify fields")
	}

	err = Rank(index, dataset, config)
	if err != nil {
		return errors.Wrap(err, "unable to rank field importance")
	}

	err = Ingest(index, dataset, config)
	if err != nil {
		return errors.Wrap(err, "unable to ingest ranked data")
	}

	return nil
}

// Merge combines all the source data files into a single datafile.
func Merge(index string, dataset string, config *IngestTaskConfig) error {
	// load the metadata from schema
	meta, err := metadata.LoadMetadataFromOriginalSchema(config.getAbsolutePath(dataset, config.SchemaPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load metadata schema")
	}

	// merge file links in dataset
	mergedDR, output, err := merge.InjectFileLinksFromFile(meta, config.getAbsolutePath(dataset, config.DataPathRelative), config.getRawDataPath(dataset), config.HasHeader)
	if err != nil {
		return errors.Wrap(err, "unable to merge linked files")
	}

	// write copy to disk
	err = ioutil.WriteFile(config.getAbsolutePath(dataset, config.MergedOutputPathRelative), output, 0644)
	if err != nil {
		return errors.Wrap(err, "unable to write merged data")
	}

	// write merged metadata out to disk
	err = meta.WriteMergedSchema(config.getAbsolutePath(dataset, config.MergedOutputSchemaPathRelative), mergedDR)
	if err != nil {
		return errors.Wrap(err, "unable to write merged schema")
	}

	return nil
}

// Classify uses the merged datafile and determines the data types of
// every variable specified in the merged schema file.
func Classify(index string, dataset string, config *IngestTaskConfig) error {
	client := rest.NewClient(config.ClassificationRESTEndpoint)

	// create classifier
	classifier := rest.NewClassifier(config.ClassificationFunctionName, client)

	// classify the file
	classification, err := classifier.ClassifyFile(config.getAbsolutePath(dataset, config.MergedOutputPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to classify dataset")
	}

	// marshall result
	bytes, err := json.MarshalIndent(classification, "", "    ")
	if err != nil {
		return errors.Wrap(err, "unable to serialize classification result")
	}
	// write to file
	err = ioutil.WriteFile(config.getAbsolutePath(dataset, config.ClassificationOutputPathRelative), bytes, 0644)
	if err != nil {
		return errors.Wrap(err, "unable to store classification result")
	}

	return nil
}

// Rank the importance of the variables in the dataset.
func Rank(index string, dataset string, config *IngestTaskConfig) error {
	// get the header for the rank data
	meta, err := metadata.LoadMetadataFromClassification(
		config.getAbsolutePath(dataset, config.MergedOutputSchemaPathRelative),
		config.getAbsolutePath(dataset, config.ClassificationOutputPathRelative))
	if err != nil {
		errors.Wrap(err, "unable to load metadata")
	}

	header, err := meta.GenerateHeaders()
	if err != nil {
		errors.Wrap(err, "unable to load metadata")
	}

	if len(header) != 1 {
		errors.Errorf("merge data should only have one header but found %d", len(header))
	}

	// need to ignore rows with missing
	// ranking requires a header
	err = removeMissingValues(config.getAbsolutePath(dataset, config.MergedOutputPathRelative), config.getAbsolutePath(dataset, rankingFilename), config.HasHeader, header[0])
	if err != nil {
		return errors.Wrap(err, "unable to ignore missing values")
	}

	// create ranker
	client := rest.NewClient(config.RankingRESTEndpoint)
	ranker := rest.NewRanker(config.RankingFunctionName, client)

	// get the importance from the REST interface
	importance, err := ranker.RankFile(config.getAbsolutePath(dataset, rankingFilename))
	if err != nil {
		return errors.Wrap(err, "unable to rank importance file")
	}

	// marshall result
	bytes, err := json.MarshalIndent(importance, "", "    ")
	if err != nil {
		return errors.Wrap(err, "unable to marshall importance ranking result")
	}

	// write to file
	outputPath := config.getAbsolutePath(dataset, config.RankingOutputPathRelative)
	err = ioutil.WriteFile(outputPath, bytes, 0644)
	if err != nil {
		return errors.Wrapf(err, "unable to write importance ranking to '%s'", outputPath)
	}

	return nil
}

// Ingest the metadata to ES and the data to Postgres.
func Ingest(index string, dataset string, config *IngestTaskConfig) error {
	meta, err := metadata.LoadMetadataFromClassification(
		config.getAbsolutePath(dataset, config.MergedOutputSchemaPathRelative),
		config.getAbsolutePath(dataset, config.ClassificationOutputPathRelative))
	if err != nil {
		errors.Wrap(err, "unable to load metadata")
	}

	indices := make([]int, len(meta.DataResources[0].Variables))
	for i := 0; i < len(indices); i++ {
		indices[i] = i
	}
	err = meta.LoadImportance(config.getAbsolutePath(dataset, config.RankingOutputPathRelative), indices)
	if err != nil {
		return errors.Wrap(err, "unable to load importance from file")
	}

	// load summary
	err = meta.LoadSummary(config.getAbsolutePath(dataset, config.SummaryOutputPathRelative), true)
	if err != nil {
		return errors.Wrap(err, "unable to load summary")
	}

	// load stats
	err = meta.LoadDatasetStats(config.getAbsolutePath(dataset, config.MergedOutputPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load stats")
	}

	// create elasticsearch client
	elasticClient, err := elastic.NewClient(
		elastic.SetURL(config.ESEndpoint),
		elastic.SetHttpClient(&http.Client{Timeout: time.Second * time.Duration(config.ESTimeout)}),
		elastic.SetMaxRetries(10),
		elastic.SetSniff(false),
		elastic.SetGzip(true))
	if err != nil {
		return errors.Wrap(err, "unable to initialize elastic client")
	}

	// ingest the metadata
	// Create the metadata index if it doesn't exist
	err = metadata.CreateMetadataIndex(elasticClient, index, false)
	if err != nil {
		return errors.Wrap(err, "unable to create metadata index")
	}

	// Ingest the dataset info into the metadata index
	err = metadata.IngestMetadata(elasticClient, index, config.ESDatasetPrefix, meta)
	if err != nil {
		return errors.Wrap(err, "unable to ingest metadata")
	}

	// Connect to the database.
	postgresConfig := &conf.Conf{
		DBPassword: config.DatabasePassword,
		DBUser:     config.DatabaseUser,
		Database:   config.Database,
	}
	pg, err := postgres.NewDatabase(postgresConfig)
	if err != nil {
		return errors.Wrap(err, "unable to initialize a new database")
	}

	dbTable := fmt.Sprintf("%s%s", config.ESDatasetPrefix, dataset)

	// Drop the current table if requested.
	pg.DropTable(dbTable)

	// Create the database table.
	ds, err := pg.InitializeDataset(meta)
	if err != nil {
		return errors.Wrap(err, "unable to initialize a new dataset")
	}

	err = pg.InitializeTable(dbTable, ds)
	if err != nil {
		return errors.Wrap(err, "unable to initialize a table")
	}

	err = pg.StoreMetadata(dbTable)
	if err != nil {
		return errors.Wrap(err, "unable to store the metadata")
	}

	err = pg.CreateResultTable(dbTable)
	if err != nil {
		return errors.Wrap(err, "unable to create the result table")
	}

	err = pg.CreatePipelineMetadataTables()
	if err != nil {
		return errors.Wrap(err, "unable to create pipeline metadata tables")
	}

	// Load the data.
	reader, err := os.Open(config.getAbsolutePath(dataset, config.MergedOutputPathRelative))
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		err = pg.IngestRow(dbTable, line)
		if err != nil {
			return errors.Wrap(err, "unable to ingest row")
		}
	}

	err = pg.InsertRemainingRows()
	if err != nil {
		return errors.Wrap(err, "unable to ingest last rows")
	}

	return nil
}

func removeMissingValues(sourceFile string, destinationFile string, hasHeader bool, headerToWrite []string) error {
	// Copy source to destination, removing rows that have missing values.
	file, err := os.Open(sourceFile)
	if err != nil {
		return errors.Wrap(err, "failed to open source file")
	}

	reader := csv.NewReader(file)

	// output writer
	output := &bytes.Buffer{}
	writer := csv.NewWriter(output)
	if headerToWrite != nil && len(headerToWrite) > 0 {
		err := writer.Write(headerToWrite)
		if err != nil {
			return errors.Wrap(err, "failed to write header to file")
		}
	}

	count := 0
	for {
		skipLine := false
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrap(err, "failed to read line from file")
		}
		if count > 0 || !hasHeader {
			for _, col := range line {
				// TODO: this is a temp fix for missing values
				if col == "" {
					skipLine = true
				}
			}
			// write the csv line back out
			if !skipLine {
				err := writer.Write(line)
				if err != nil {
					return errors.Wrap(err, "failed to write line to file")
				}
			}
		}
		count++
	}
	// flush writer
	writer.Flush()

	err = ioutil.WriteFile(destinationFile, output.Bytes(), 0644)
	if err != nil {
		return errors.Wrap(err, "failed to close output file")
	}

	// close left
	err = file.Close()
	if err != nil {
		return errors.Wrap(err, "failed to close input file")
	}
	return nil
}
