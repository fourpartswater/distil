package task

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/unchartedsoftware/plog"
	"gopkg.in/olivere/elastic.v5"

	"github.com/unchartedsoftware/distil-ingest/conf"
	"github.com/unchartedsoftware/distil-ingest/feature"
	"github.com/unchartedsoftware/distil-ingest/merge"
	"github.com/unchartedsoftware/distil-ingest/metadata"
	"github.com/unchartedsoftware/distil-ingest/postgres"
	"github.com/unchartedsoftware/distil-ingest/rest"
	"github.com/unchartedsoftware/distil/api/model"
)

const (
	rankingFilename = "rank-no-missing.csv"
	baseTableSuffix = "_base"
	datasetIDSuffix = "_dataset"
)

// IngestTaskConfig captures the necessary configuration for an data ingest.
type IngestTaskConfig struct {
	ContainerDataPath                  string
	TmpDataPath                        string
	DataPathRelative                   string
	DatasetFolderSuffix                string
	MediaPathRelative                  string
	HasHeader                          bool
	FeaturizationRESTEndpoint          string
	FeaturizationFunctionName          string
	FeaturizationOutputDataRelative    string
	FeaturizationOutputSchemaRelative  string
	MergedOutputPathRelative           string
	MergedOutputSchemaPathRelative     string
	SchemaPathRelative                 string
	ClassificationRESTEndpoint         string
	ClassificationFunctionName         string
	ClassificationOutputPathRelative   string
	ClassificationProbabilityThreshold float64
	RankingRESTEndpoint                string
	RankingFunctionName                string
	RankingOutputPathRelative          string
	RankingRowLimit                    int
	DatabasePassword                   string
	DatabaseUser                       string
	Database                           string
	DatabaseHost                       string
	DatabasePort                       int
	SummaryOutputPathRelative          string
	SummaryMachineOutputPathRelative   string
	SummaryRESTEndpoint                string
	SummaryFunctionName                string
	ESEndpoint                         string
	ESTimeout                          int
	ESDatasetPrefix                    string
}

func (c *IngestTaskConfig) getRootPath(dataset string) string {
	return fmt.Sprintf("%s/%s/%s%s", c.ContainerDataPath, dataset, dataset, c.DatasetFolderSuffix)
}

func (c *IngestTaskConfig) getAbsolutePath(relativePath string) string {
	return fmt.Sprintf("%s/%s", c.ContainerDataPath, relativePath)
}

func (c *IngestTaskConfig) getTmpAbsolutePath(relativePath string) string {
	return fmt.Sprintf("%s/%s", c.TmpDataPath, relativePath)
}

func (c *IngestTaskConfig) getRawDataPath() string {
	return fmt.Sprintf("%s/", c.ContainerDataPath)
}

// IngestDataset executes the complete ingest process for the specified dataset.
func IngestDataset(metaCtor model.MetadataStorageCtor, index string, dataset string, config *IngestTaskConfig) error {
	// Make sure the temp data directory exists.
	tmpPath := path.Dir(config.getTmpAbsolutePath(config.MergedOutputSchemaPathRelative))
	os.MkdirAll(tmpPath, os.ModePerm)

	// Set the probability threshold
	metadata.SetTypeProbabilityThreshold(config.ClassificationProbabilityThreshold)

	storage, err := metaCtor()
	if err != nil {
		return errors.Wrap(err, "unable to initialize metadata storage")
	}

	err = Featurize(index, dataset, config)
	if err != nil {
		return errors.Wrap(err, "unable to featurize all data")
	}

	err = Merge(index, dataset, config)
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

	err = Summarize(index, dataset, config)
	// NOTE: For now ignore summary errors!
	//if err != nil {
	//	return errors.Wrap(err, "unable to summarize the dataset")
	//}

	err = Ingest(storage, index, dataset, config)
	if err != nil {
		return errors.Wrap(err, "unable to ingest ranked data")
	}

	return nil
}

// Featurize uses primitives to obtain a featurized view of complex variables.
func Featurize(index string, dataset string, config *IngestTaskConfig) error {
	client := rest.NewClient(config.FeaturizationRESTEndpoint)

	// create featurizer
	featurizer := rest.NewFeaturizer(config.FeaturizationFunctionName, client)

	// load metadata from original schema
	meta, err := metadata.LoadMetadataFromOriginalSchema(config.getAbsolutePath(config.SchemaPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load original schema file")
	}

	// featurize data
	err = feature.FeaturizeDataset(meta, featurizer, config.getAbsolutePath(config.DataPathRelative),
		config.getAbsolutePath(config.MediaPathRelative), config.getAbsolutePath(config.TmpDataPath),
		config.getAbsolutePath(config.FeaturizationOutputDataRelative),
		config.getAbsolutePath(config.FeaturizationOutputSchemaRelative), config.HasHeader)
	if err != nil {
		return errors.Wrap(err, "unable to featurize data")
	}

	log.Infof("Featurized data written to %s", config.getAbsolutePath(config.TmpDataPath))

	return nil
}

// Merge combines all the source data files into a single datafile.
func Merge(index string, dataset string, config *IngestTaskConfig) error {
	// load the metadata from schema
	meta, err := metadata.LoadMetadataFromOriginalSchema(config.getAbsolutePath(config.FeaturizationOutputSchemaRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load metadata schema")
	}

	// merge file links in dataset
	mergedDR, output, err := merge.InjectFileLinksFromFile(meta, config.getAbsolutePath(config.FeaturizationOutputDataRelative), config.getRawDataPath(), config.getAbsolutePath(config.MergedOutputPathRelative), config.HasHeader)
	if err != nil {
		return errors.Wrap(err, "unable to merge linked files")
	}

	// write copy to disk
	err = ioutil.WriteFile(config.getTmpAbsolutePath(config.MergedOutputPathRelative), output, 0644)
	if err != nil {
		return errors.Wrap(err, "unable to write merged data")
	}

	// write merged metadata out to disk
	err = meta.WriteMergedSchema(config.getTmpAbsolutePath(config.MergedOutputSchemaPathRelative), mergedDR)
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
	classification, err := classifier.ClassifyFile(config.getTmpAbsolutePath(config.MergedOutputPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to classify dataset")
	}

	// marshall result
	bytes, err := json.MarshalIndent(classification, "", "    ")
	if err != nil {
		return errors.Wrap(err, "unable to serialize classification result")
	}
	// write to file
	err = ioutil.WriteFile(config.getTmpAbsolutePath(config.ClassificationOutputPathRelative), bytes, 0644)
	if err != nil {
		return errors.Wrap(err, "unable to store classification result")
	}

	return nil
}

// Rank the importance of the variables in the dataset.
func Rank(index string, dataset string, config *IngestTaskConfig) error {
	// get the header for the rank data
	meta, err := metadata.LoadMetadataFromClassification(
		config.getTmpAbsolutePath(config.MergedOutputSchemaPathRelative),
		config.getTmpAbsolutePath(config.ClassificationOutputPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load metadata")
	}

	header, err := meta.GenerateHeaders()
	if err != nil {
		return errors.Wrap(err, "unable to load metadata")
	}

	if len(header) != 1 {
		return errors.Errorf("merge data should only have one header but found %d", len(header))
	}

	// need to ignore rows with missing
	// ranking requires a header
	err = removeMissingValues(
		config.getTmpAbsolutePath(config.MergedOutputPathRelative),
		config.getTmpAbsolutePath(rankingFilename),
		config.HasHeader, header[0], config.RankingRowLimit)
	if err != nil {
		return errors.Wrap(err, "unable to ignore missing values")
	}

	// create ranker
	client := rest.NewClient(config.RankingRESTEndpoint)
	ranker := rest.NewRanker(config.RankingFunctionName, client)

	// get the importance from the REST interface
	importance, err := ranker.RankFile(config.getTmpAbsolutePath(rankingFilename))
	if err != nil {
		return errors.Wrap(err, "unable to rank importance file")
	}

	// marshall result
	bytes, err := json.MarshalIndent(importance, "", "    ")
	if err != nil {
		return errors.Wrap(err, "unable to marshall importance ranking result")
	}

	// write to file
	outputPath := config.getTmpAbsolutePath(config.RankingOutputPathRelative)
	err = ioutil.WriteFile(outputPath, bytes, 0644)
	if err != nil {
		return errors.Wrapf(err, "unable to write importance ranking to '%s'", outputPath)
	}

	return nil
}

// Summarize the contents of the dataset.
func Summarize(index string, dataset string, config *IngestTaskConfig) error {
	// create ranker
	client := rest.NewClient(config.SummaryRESTEndpoint)
	summarizer := rest.NewSummarizer(config.SummaryFunctionName, client)

	// get the importance from the REST interface
	summary, err := summarizer.SummarizeFile(config.getTmpAbsolutePath(config.MergedOutputPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to summarize merged file")
	}

	// marshall result
	bytes, err := json.MarshalIndent(summary, "", "    ")
	if err != nil {
		return errors.Wrap(err, "unable to marshall summary result")
	}

	// write to file
	outputPath := config.getTmpAbsolutePath(config.SummaryMachineOutputPathRelative)
	err = ioutil.WriteFile(outputPath, bytes, 0644)
	if err != nil {
		return errors.Wrapf(err, "unable to write summary to '%s'", outputPath)
	}

	return nil
}

// Ingest the metadata to ES and the data to Postgres.
func Ingest(storage model.MetadataStorage, index string, dataset string, config *IngestTaskConfig) error {
	meta, err := metadata.LoadMetadataFromClassification(
		config.getTmpAbsolutePath(config.MergedOutputSchemaPathRelative),
		config.getTmpAbsolutePath(config.ClassificationOutputPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load metadata")
	}

	// Adjust the ID & name of the dataset as needed
	fixDatasetIDName(meta)

	indices := make([]int, len(meta.DataResources[0].Variables))
	for i := 0; i < len(indices); i++ {
		indices[i] = i
	}
	err = meta.LoadImportance(config.getTmpAbsolutePath(config.RankingOutputPathRelative), indices)
	if err != nil {
		return errors.Wrap(err, "unable to load importance from file")
	}

	// load stats
	err = meta.LoadDatasetStats(config.getTmpAbsolutePath(config.MergedOutputPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load stats")
	}

	// load summary
	err = meta.LoadSummaryFromDescription(config.getTmpAbsolutePath(config.SummaryOutputPathRelative))
	if err != nil {
		return errors.Wrap(err, "unable to load summary")
	}

	// load stats
	err = meta.LoadSummaryMachine(config.getTmpAbsolutePath(config.SummaryMachineOutputPathRelative))
	// NOTE: For now ignore summary errors!
	//if err != nil {
	//	return errors.Wrap(err, "unable to load stats")
	//}

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

	// Connect to the database.
	postgresConfig := &conf.Conf{
		DBPassword: config.DatabasePassword,
		DBUser:     config.DatabaseUser,
		Database:   config.Database,
		DBHost:     config.DatabaseHost,
		DBPort:     config.DatabasePort,
	}
	pg, err := postgres.NewDatabase(postgresConfig)
	if err != nil {
		return errors.Wrap(err, "unable to initialize a new database")
	}

	// Check for existing dataset
	match, err := matchDataset(storage, meta, index)
	// Ignore the error for now as if this fails we still want ingest to succeed.
	if err != nil {
		log.Error(err)
	}
	if match != "" {
		log.Infof("Matched %s to dataset %s", meta.Name, match)
		err = deleteDataset(match, index, pg, elasticClient)
		log.Infof("Deleted dataset %s", match)
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

	dbTable := strings.Replace(meta.ID, datasetIDSuffix, "", -1)
	dbTable = fmt.Sprintf("%s%s", config.ESDatasetPrefix, dbTable)

	// Drop the current table if requested.
	// Hardcoded the base table name for now.
	pg.DropView(dbTable)
	pg.DropTable(fmt.Sprintf("%s%s", dbTable, baseTableSuffix))

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

	err = pg.CreateSolutionMetadataTables()
	if err != nil {
		return errors.Wrap(err, "unable to create solution metadata tables")
	}

	// Load the data.
	reader, err := os.Open(config.getTmpAbsolutePath(config.MergedOutputPathRelative))
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		err = pg.AddWordStems(line)
		if err != nil {
			log.Warn(fmt.Sprintf("%v", err))
		}

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

func removeMissingValues(sourceFile string, destinationFile string, hasHeader bool, headerToWrite []string, rowLimit int) error {
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
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrap(err, "failed to read line from file")
		}
		if (count > 0 || !hasHeader) && (count < rowLimit) {
			// write the csv line back out
			err := writer.Write(line)
			if err != nil {
				return errors.Wrap(err, "failed to write line to file")
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

func fixDatasetIDName(meta *metadata.Metadata) {
	// Train dataset ID & name need to be adjusted to fit the expected format.
	// The ID MUST end in _dataset, and the name should be representative.
	if isTrainDataset(meta) {
		meta.ID = strings.TrimSuffix(meta.ID, "_TRAIN")
		if !strings.HasSuffix(meta.ID, datasetIDSuffix) {
			meta.ID = fmt.Sprintf("%s%s", meta.ID, datasetIDSuffix)
		}
		meta.Name = strings.TrimSuffix(meta.ID, datasetIDSuffix)
	}
}

func isTrainDataset(meta *metadata.Metadata) bool {
	return strings.HasSuffix(meta.ID, "_TRAIN")
}

func matchDataset(storage model.MetadataStorage, meta *metadata.Metadata, index string) (string, error) {
	// load the datasets from ES.
	datasets, err := storage.FetchDatasets(true)
	if err != nil {
		return "", errors.Wrap(err, "unable to fetch datasets for matching")
	}

	// See if any of the loaded datasets match.
	for _, dataset := range datasets {
		variables := make([]string, 0)
		for _, v := range dataset.Variables {
			variables = append(variables, v.Name)
		}
		if meta.DatasetMatches(variables) {
			// Return the name of the matching set.
			return dataset.Name, nil
		}
	}

	// No matching set.
	return "", nil
}

func deleteDataset(name string, index string, pg *postgres.Database, es *elastic.Client) error {
	id := fmt.Sprintf("%s%s", name, datasetIDSuffix)
	success := false
	for i := 0; i < 10 && !success; i++ {
		_, err := es.Delete().Index(index).Id(id).Type("metadata").Do(context.Background())
		if err != nil {
			log.Error(err)
		} else {
			success = true
		}
	}

	if success {
		pg.DeleteDataset(name)
	}

	return nil
}
