package compute

import (
	"sync"

	"github.com/pkg/errors"
	"github.com/unchartedsoftware/distil/api/middleware"
	"github.com/unchartedsoftware/distil/api/pipeline"
	"github.com/unchartedsoftware/plog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Client provides facilities for managing GPRC solution requests. Requests are
// isssued and a context object containing rx channels is returned to the caller for consumption
// of results. The context for running requests can also be fetched, along with their buffered
// results. Spawning a grpc.ClientConn per RPC call is not considered good practice - the system
// is designed such that multiple go routines make RPC calls to a single shared client, and synch
// is managed internally.
type Client struct {
	client    pipeline.CoreClient
	conn      *grpc.ClientConn
	mu        *sync.Mutex
	DataDir   string
	UserAgent string
}

// NewClient creates a new pipline request dispatcher instance. This will establish
// the connection to the solution server or return an error on fail
func NewClient(serverAddr string, dataDir string, trace bool, userAgent string) (*Client, error) {
	conn, err := grpc.Dial(
		serverAddr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(middleware.GenerateUnaryClientInterceptor(trace)),
		grpc.WithStreamInterceptor(middleware.GenerateStreamClientInterceptor(trace)),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to %s", serverAddr)
	}
	log.Infof("connected to %s", serverAddr)

	client := Client{}
	client.client = pipeline.NewCoreClient(conn)
	client.conn = conn
	client.DataDir = dataDir
	client.UserAgent = userAgent
	return &client, nil
}

// Close the connection to the solution service
func (c *Client) Close() {
	log.Infof("client connection closed")
	c.conn.Close()
}

// StartSearch starts a solution search session.
func (c *Client) StartSearch(ctx context.Context, request *pipeline.SearchSolutionsRequest) (string, error) {

	searchSolutionResponse, err := c.client.SearchSolutions(ctx, request)
	if err != nil {
		return "", err
	}

	return searchSolutionResponse.SearchId, nil
}

// SearchSolutions generates candidate pipel\ines.
func (c *Client) SearchSolutions(ctx context.Context, searchID string) ([]*pipeline.GetSearchSolutionsResultsResponse, error) {

	searchPiplinesResultsRequest := &pipeline.GetSearchSolutionsResultsRequest{
		SearchId: searchID,
	}

	searchSolutionsResultsResponse, err := c.client.GetSearchSolutionsResults(ctx, searchPiplinesResultsRequest)
	if err != nil {
		return nil, err
	}

	var solutionResultResponses []*pipeline.GetSearchSolutionsResultsResponse

	err = pullFromAPI(pullMax, pullTimeout, func() error {
		solutionResultResponse, err := searchSolutionsResultsResponse.Recv()
		if err != nil {
			return err
		}
		solutionResultResponses = append(solutionResultResponses, solutionResultResponse)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return solutionResultResponses, nil
}

// GenerateSolutionScores generates scrores for candidate solutions.
func (c *Client) GenerateSolutionScores(ctx context.Context, solutionID string) ([]*pipeline.GetScoreSolutionResultsResponse, error) {

	scoreSolutionRequest := &pipeline.ScoreSolutionRequest{
		SolutionId: solutionID,
	}

	scoreSolutionResponse, err := c.client.ScoreSolution(ctx, scoreSolutionRequest)
	if err != nil {
		return nil, err
	}

	searchPiplinesResultsRequest := &pipeline.GetScoreSolutionResultsRequest{
		RequestId: scoreSolutionResponse.RequestId,
	}

	scoreSolutionResultsResponse, err := c.client.GetScoreSolutionResults(ctx, searchPiplinesResultsRequest)
	if err != nil {
		return nil, err
	}

	var solutionResultResponses []*pipeline.GetScoreSolutionResultsResponse

	err = pullFromAPI(pullMax, pullTimeout, func() error {
		solutionResultResponse, err := scoreSolutionResultsResponse.Recv()
		if err != nil {
			return err
		}
		solutionResultResponses = append(solutionResultResponses, solutionResultResponse)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return solutionResultResponses, nil
}

// GenerateSolutionFit generates fit for candidate solutions.
func (c *Client) GenerateSolutionFit(ctx context.Context, solutionID string, datasetURI string) ([]*pipeline.GetFitSolutionResultsResponse, error) {

	fitSolutionRequest := &pipeline.FitSolutionRequest{
		SolutionId: solutionID,
		Inputs: []*pipeline.Value{
			{
				Value: &pipeline.Value_DatasetUri{
					DatasetUri: datasetURI,
				},
			},
		},
	}

	fitSolutionResponse, err := c.client.FitSolution(ctx, fitSolutionRequest)
	if err != nil {
		return nil, err
	}

	fitSolutionResultsRequest := &pipeline.GetFitSolutionResultsRequest{
		RequestId: fitSolutionResponse.RequestId,
	}

	fitSolutionResultsResponse, err := c.client.GetFitSolutionResults(ctx, fitSolutionResultsRequest)
	if err != nil {
		return nil, err
	}

	var solutionResultResponses []*pipeline.GetFitSolutionResultsResponse

	err = pullFromAPI(pullMax, pullTimeout, func() error {
		solutionResultResponse, err := fitSolutionResultsResponse.Recv()
		if err != nil {
			return err
		}
		solutionResultResponses = append(solutionResultResponses, solutionResultResponse)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return solutionResultResponses, nil
}

// GeneratePredictions generates predictions.
func (c *Client) GeneratePredictions(ctx context.Context, request *pipeline.ProduceSolutionRequest) ([]*pipeline.GetProduceSolutionResultsResponse, error) {

	produceSolutionResponse, err := c.client.ProduceSolution(ctx, request)
	if err != nil {
		return nil, err
	}

	produceSolutionResultsRequest := &pipeline.GetProduceSolutionResultsRequest{
		RequestId: produceSolutionResponse.RequestId,
	}

	produceSolutionResultsResponse, err := c.client.GetProduceSolutionResults(ctx, produceSolutionResultsRequest)
	if err != nil {
		return nil, err
	}

	var solutionResultResponses []*pipeline.GetProduceSolutionResultsResponse

	err = pullFromAPI(pullMax, pullTimeout, func() error {
		solutionResultResponse, err := produceSolutionResultsResponse.Recv()
		if err != nil {
			return err
		}
		solutionResultResponses = append(solutionResultResponses, solutionResultResponse)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return solutionResultResponses, nil
}

// EndSearch ends the solution search session.
func (c *Client) EndSearch(ctx context.Context, searchID string) error {

	endSearchSolutions := &pipeline.EndSearchSolutionsRequest{
		SearchId: searchID,
	}

	_, err := c.client.EndSearchSolutions(ctx, endSearchSolutions)
	return err
}

// ExportSolution exports the solution.
func (c *Client) ExportSolution(ctx context.Context, solutionID string, exportURI string) error {
	return nil
}