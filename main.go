package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/unchartedsoftware/plog"
	"github.com/zenazn/goji/graceful"
	"goji.io"
	"goji.io/pat"

	"github.com/unchartedsoftware/distil/api/elastic"
	"github.com/unchartedsoftware/distil/api/env"
	"github.com/unchartedsoftware/distil/api/middleware"
	pg "github.com/unchartedsoftware/distil/api/model/storage/postgres"
	"github.com/unchartedsoftware/distil/api/pipeline"
	"github.com/unchartedsoftware/distil/api/postgres"
	"github.com/unchartedsoftware/distil/api/redis"
	"github.com/unchartedsoftware/distil/api/routes"
	"github.com/unchartedsoftware/distil/api/ws"
)

const (
	defaultEsEndpoint              = "http://localhost:9200"
	defaultRedisEndpoint           = "localhost:6379"
	defaultRedisExpiry             = -1 // no expiry
	defaultAppPort                 = "8080"
	defaultPipelineComputeEndPoint = "localhost:9500"
	defaultPipelineComputeTrace    = "false"
	defaultPipelineDataDir         = "datasets"
	defaultPGStorage               = "true"
	defaultPGHost                  = "localhost"
	defaultPGPort                  = "5432"
	defaultPGUser                  = "distil"
	defaultPGPassword              = ""
	defaultPGDatabase              = "distil"
	defaultPGRetries               = 100
	deafultPGRetryTimeout          = 4000
	defaultStartupConfigFile       = "startup.json"
)

var (
	version   = "unset"
	timestamp = "unset"
)

func registerRoute(mux *goji.Mux, pattern string, handler func(http.ResponseWriter, *http.Request)) {
	log.Infof("Registering route %s", pattern)
	mux.HandleFunc(pat.Get(pattern), handler)
}

func main() {
	log.Infof("version: %s built: %s", version, timestamp)

	// load elasticsearch endpoint
	esEndpoint := env.Load("ES_ENDPOINT", defaultEsEndpoint)
	// load application port
	redisEndpoint := env.Load("REDIS_ENDPOINT", defaultRedisEndpoint)
	// load redis endpoint
	httpPort := env.Load("PORT", defaultAppPort)
	// load compute server endpoint
	pipelineComputeEndpoint := env.Load("PIPELINE_COMPUTE_ENDPOINT", defaultPipelineComputeEndPoint)

	// load default temp dataset directory
	pipelineDataDir := env.Load("PIPELINE_DATA_DIR", defaultPipelineDataDir)
	// load trace log enable state
	traceEnv := env.Load("PIPELINE_COMPUTE_TRACE", defaultPipelineComputeTrace)
	pipelineComputeTrace, err := strconv.ParseBool(traceEnv)
	if err != nil {
		log.Warnf("Failed to parse PIPELINE_COMPUTE_TRACE as bool: %v", err)
		pipelineComputeTrace = false
	}

	// instantiate elastic client constructor.
	esClientCtor := elastic.NewClient(esEndpoint, false)

	// read startup config
	startupConfigFile := env.Load("CONFIG_JSON_PATH", defaultStartupConfigFile)
	startupConfig, err := ioutil.ReadFile(startupConfigFile)
	exportPath := ""
	if err != nil {
		log.Warnf("Failed to read startup config file (%s): %v", startupConfigFile, err)
	} else {
		var startupData map[string]interface{}
		err = json.Unmarshal(startupConfig, &startupData)
		if err != nil {
			log.Warnf("Failed to parse startup config file (%s): %v", startupConfigFile, err)
		} else {
			exportPath = startupData["executables_root"].(string)
			log.Infof("executables_root = %s, from config json", exportPath)
			pipelineDataDir = startupData["temp_storage_root"].(string)
			log.Infof("temp_storage_root = %s, from config json - overrides PIPELINE_DATA_DIR", pipelineDataDir)
		}
	}

	// load the postgres parameters.
	pgHost := env.Load("PG_HOST", defaultPGHost)
	pgPort := env.Load("PG_PORT", defaultPGPort)
	pgUser := env.Load("PG_USER", defaultPGUser)
	pgPassword := env.Load("PG_PASSWORD", defaultPGPassword)
	pgDatabase := env.Load("PG_DATABASE", defaultPGDatabase)

	// instantiate the postgres client constructor.
	postgresClientCtor := postgres.NewClient(pgHost, pgPort, pgUser, pgPassword, pgDatabase)

	// make sure a connection can be made to postgres - doesn't appear to be thread safe and
	// causes panic if deferred, so we'll do it an a retry loop here.  We need to provide
	// flexibility on startup because we can't guarantee the DB will be up before the server.
	for i := 0; i < defaultPGRetries; i++ {
		_, err = postgresClientCtor()
		if err == nil {
			break
		} else if i == defaultPGRetries {
			log.Errorf("%v", err)
			os.Exit(1)
		}
		log.Errorf("%v", err)
		time.Sleep(deafultPGRetryTimeout * time.Millisecond)
	}

	// instantiate the postgres storage constructor.
	pgStorageCtor := pg.NewStorage(postgresClientCtor, esClientCtor)

	dataStorageCtor := pgStorageCtor

	// Instantiate the pipeline compute client
	pipelineClient, err := pipeline.NewClient(pipelineComputeEndpoint, pipelineDataDir, pipelineComputeTrace)
	if err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
	defer pipelineClient.Close()

	// instantiate redis pool
	redisPool := redis.NewPool(redisEndpoint, defaultRedisExpiry)

	// register routes
	mux := goji.NewMux()

	mux.Use(middleware.Log)
	mux.Use(middleware.Gzip)
	mux.Use(middleware.Redis(redisPool))

	registerRoute(mux, "/distil/datasets/:index", routes.DatasetsHandler(esClientCtor))
	registerRoute(mux, "/distil/variables/:index/:dataset", routes.VariablesHandler(esClientCtor))
	registerRoute(mux, "/distil/variable-summaries/:index/:dataset/:variable", routes.VariableSummaryHandler(dataStorageCtor, esClientCtor))
	registerRoute(mux, "/distil/filtered-data/:esIndex/:dataset/:inclusive", routes.FilteredDataHandler(dataStorageCtor))
	registerRoute(mux, "/distil/results/:index/:dataset/:results-uuid/:inclusive", routes.ResultsHandler(dataStorageCtor))
	registerRoute(mux, "/distil/results-summary/:index/:dataset/:results-uuid", routes.ResultsSummaryHandler(dataStorageCtor))
	registerRoute(mux, "/distil/session/:session", routes.SessionHandler(dataStorageCtor))
	registerRoute(mux, "/distil/abort", routes.AbortHandler())
	registerRoute(mux, "/distil/export/:session/:pipeline-id", routes.ExportHandler(pipelineClient, exportPath))

	registerRoute(mux, "/ws", ws.PipelineHandler(pipelineClient, esClientCtor, dataStorageCtor))
	registerRoute(mux, "/*", routes.FileHandler("./dist"))

	// catch kill signals for graceful shutdown
	graceful.AddSignal(syscall.SIGINT, syscall.SIGTERM)

	// kick off the server listen loop
	log.Infof("Listening on port %s", httpPort)
	err = graceful.ListenAndServe(":"+httpPort, mux)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// wait until server gracefully exits
	graceful.Wait()
}
