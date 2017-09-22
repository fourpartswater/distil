package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/unchartedsoftware/distil/api/util/json"
	"github.com/unchartedsoftware/distil/api/util/mock"
)

func TestDatasetsHandler(t *testing.T) {
	// mock elasticsearch request handler
	handler := mock.ElasticHandler(t, []string{
		"./testdata/datasets.json",
		"./testdata/stats.json",
		"./testdata/stats.json",
	})
	// mock elasticsearch client
	ctor := mock.ElasticClientCtor(t, handler)

	// put together a stub dataset request
	req := mock.HTTPRequest(t, "GET", "/distil/datasets/", map[string]string{
		"index": "datasets",
	}, nil)

	// execute the test request - stubbed ES server will return the JSON
	// loaded above
	res := mock.HTTPResponse(t, req, DatasetsHandler(ctor))
	assert.Equal(t, http.StatusOK, res.Code)

	// compare expected and acutal results - unmarshall first to ensure object
	// rather than byte equality
	expected, err := json.Unmarshal([]byte(
		`{
			"datasets": [
				{
					"name": "o_185",
					"description": "<p><strong>Author</strong>: Jeffrey S. Simonoff</p>\n",
					"summary": "",
					"numRows": 1073,
					"numBytes": 744647,
					"variables": [
						{"name":"d3mIndex","type":"integer","importance": 0},
						{"name":"Player","type":"categorical","importance": 0},
						{"name":"Number_seasons","type":"integer","importance": 1},
						{"name":"Games_played","type":"integer","importance": 2}
					]
				},
				{
					"name": "o_196",
					"description": "<p><strong>Author</strong>: Mr. Somebody</p>\n",
					"summary": "",
					"numRows": 1073,
					"numBytes": 744647,
					"variables": [
						{"name":"d3mIndex","type":"integer","importance": 0},
						{"name":"cylinders","type":"categorical","importance": 0},
						{"name":"displacement","type":"categorical","importance": 0}
					]
				}
			]
		}`))
	assert.NoError(t, err)

	actual, err := json.Unmarshal(res.Body.Bytes())
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}

func TestDatasetsHandlerWithSearch(t *testing.T) {
	// mock elasticsearch request handler
	handler := mock.ElasticHandler(t, []string{
		"./testdata/search.json",
		"./testdata/stats.json",
		"./testdata/stats.json",
	})
	// mock elasticsearch client
	ctor := mock.ElasticClientCtor(t, handler)

	// put together a stub dataset request
	params := map[string]string{
		"index": "datasets",
	}
	query := map[string]string{
		"search": "baseball",
	}
	req := mock.HTTPRequest(t, "GET", "/distil/datasets?search=baseball", params, query)

	// execute the test request - stubbed ES server will return the JSON
	// loaded above
	res := mock.HTTPResponse(t, req, DatasetsHandler(ctor))
	assert.Equal(t, http.StatusOK, res.Code)

	// compare expected and actual results - unmarshall first to ensure object
	// rather than byte equality
	expected, err := json.Unmarshal([]byte(
		`{
			"datasets": [
				{
					"name": "o_185",
					"description": "<p><strong>Author</strong>: Jeffrey S. Simonoff</p>\n",
					"summary": "",
					"numRows": 1073,
					"numBytes": 744647,
					"variables": [
						{"name":"d3mIndex","type":"integer","importance": 0},
						{"name":"Player","type":"categorical","importance": 0},
						{"name":"Number_seasons","type":"integer","importance": 1},
						{"name":"Games_played","type":"integer","importance": 2}
					]
				},
				{
					"name": "o_196",
					"description": "<p><strong>Author</strong>: Mr. Somebody</p>\n",
					"summary": "",
					"numRows": 1073,
					"numBytes": 744647,
					"variables": [
						{"name":"d3mIndex","type":"integer","importance": 0},
						{"name":"cylinders","type":"categorical","importance": 0},
						{"name":"displacement","type":"categorical","importance": 0}
					]
				}
			]
			}`))
	assert.NoError(t, err)

	actual, err := json.Unmarshal(res.Body.Bytes())
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}
