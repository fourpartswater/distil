package routes

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/unchartedsoftware/distil/api/model"
	"github.com/unchartedsoftware/distil/api/model/storage/elastic"
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
	// mock elasticsearch client & storage
	ctor := mock.ElasticClientCtor(t, handler)
	ctorStorage := elastic.NewMetadataStorage("datasets", ctor)

	// put together a stub dataset request
	req := mock.HTTPRequest(t, "GET", "/distil/datasets/", map[string]string{
		"index": "datasets",
	}, nil)

	// execute the test request - stubbed ES server will return the JSON
	// loaded above
	res := mock.HTTPResponse(t, req, DatasetsHandler([]model.MetadataStorageCtor{ctorStorage}))
	assert.Equal(t, http.StatusOK, res.Code)

	// compare expected and acutal results - unmarshall first to ensure object
	// rather than byte equality
	expected, err := json.Unmarshal([]byte(
		`{
			"datasets": [
				{
					"name": "o_185_dataset",
					"description": "<p><strong>Author</strong>: Jeffrey S. Simonoff</p>\n",
					"summary": "",
					"summaryML": "",
					"folder":"",
					"numRows": 1073,
					"numBytes": 744647,
					"provenance": "elastic",
					"variables": [
						{"colName":"d3mIndex","colType":"integer","importance": 0,"deleted": false,"selectedRole": "index","suggestedTypes": [{ "type": "integer", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 0, "colOriginalType":"integer", "role": ["TEST"], "colDisplayName": "d3mIndex"},
						{"colName":"Player","colType":"categorical","importance": 0,"deleted": false,"selectedRole": "attribute","suggestedTypes": [{ "type": "categorical", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 1, "colOriginalType":"categorical", "role": ["TEST"], "colDisplayName": "Player"},
						{"colName":"Number_seasons","colType":"integer","importance": 1,"deleted": false,"selectedRole": "attribute","suggestedTypes":[ { "type": "integer", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 2, "colOriginalType":"integer", "role": ["TEST"], "colDisplayName": "Number_seasons"},
						{"colName":"Games_played","colType":"integer","importance": 2,"deleted": false,"selectedRole": "attribute","suggestedTypes": [{ "type": "integer", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 3, "colOriginalType":"integer", "role": ["TEST"], "colDisplayName": "Games_played"}
					]
				},
				{
					"name": "o_196_dataset",
					"description": "<p><strong>Author</strong>: Mr. Somebody</p>\n",
					"summary": "",
					"summaryML": "",
					"folder":"",
					"numRows": 1073,
					"numBytes": 744647,
					"provenance": "elastic",
					"variables": [
						{"colName":"d3mIndex","colType":"integer","importance": 0,"deleted": false,"selectedRole": "index","suggestedTypes": [{ "type": "integer", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 0, "colOriginalType":"integer", "role": ["TEST"], "colDisplayName": "d3mIndex"},
						{"colName":"cylinders","colType":"categorical","importance": 0,"deleted": false,"selectedRole": "attribute","suggestedTypes":  [{ "type": "categorical", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 1, "colOriginalType":"categorical", "role": ["TEST"], "colDisplayName": "cylinders"},
						{"colName":"displacement","colType":"categorical","importance": 0,"deleted": false,"selectedRole": "attribute","suggestedTypes":  [{ "type": "categorical", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 2, "colOriginalType":"categorical", "role": ["TEST"], "colDisplayName": "displacement"}
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
	// mock elasticsearch client & storage
	ctor := mock.ElasticClientCtor(t, handler)
	ctorStorage := elastic.NewMetadataStorage("datasets", ctor)

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
	res := mock.HTTPResponse(t, req, DatasetsHandler([]model.MetadataStorageCtor{ctorStorage}))
	assert.Equal(t, http.StatusOK, res.Code)

	// compare expected and actual results - unmarshall first to ensure object
	// rather than byte equality
	expected, err := json.Unmarshal([]byte(
		`{
			"datasets": [
				{
					"name": "o_185_dataset",
					"description": "<p><strong>Author</strong>: Jeffrey S. Simonoff</p>\n",
					"summary": "",
					"summaryML": "",
					"folder":"",
					"numRows": 1073,
					"numBytes": 744647,
					"provenance": "elastic",
					"variables": [
						{"colName":"d3mIndex","colType":"integer","importance": 0,"deleted": false,"selectedRole": "index","suggestedTypes": [{ "type": "integer", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 0, "colOriginalType":"integer", "role": ["TEST"], "colDisplayName": "d3mIndex"},
						{"colName":"Player","colType":"categorical","importance": 0,"deleted": false,"selectedRole": "attribute","suggestedTypes": [{ "type": "categorical", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 1, "colOriginalType":"categorical", "role": ["TEST"], "colDisplayName": "Player"},
						{"colName":"Number_seasons","colType":"integer","importance": 1,"deleted": false,"selectedRole": "attribute","suggestedTypes":[ { "type": "integer", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 2, "colOriginalType":"integer", "role": ["TEST"], "colDisplayName": "Number_seasons"},
						{"colName":"Games_played","colType":"integer","importance": 2,"deleted": false,"selectedRole": "attribute","suggestedTypes": [{ "type": "integer", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 3, "colOriginalType":"integer", "role": ["TEST"], "colDisplayName": "Games_played"}
					]
				},
				{
					"name": "o_196_dataset",
					"description": "<p><strong>Author</strong>: Mr. Somebody</p>\n",
					"summary": "",
					"summaryML": "",
					"folder":"",
					"numRows": 1073,
					"numBytes": 744647,
					"provenance": "elastic",
					"variables": [
						{"colName":"d3mIndex","colType":"integer","importance": 0,"deleted": false,"selectedRole": "index","suggestedTypes": [{ "type": "integer", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 0, "colOriginalType":"integer", "role": ["TEST"], "colDisplayName": "d3mIndex"},
						{"colName":"cylinders","colType":"categorical","importance": 0,"deleted": false,"selectedRole": "attribute","suggestedTypes":  [{ "type": "categorical", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 1, "colOriginalType":"categorical", "role": ["TEST"], "colDisplayName": "cylinders"},
						{"colName":"displacement","colType":"categorical","importance": 0,"deleted": false,"selectedRole": "attribute","suggestedTypes":  [{ "type": "categorical", "probability": 1.00, "provenance": "TEST" }], "colOriginalVariable": "","colIndex": 2, "colOriginalType":"categorical", "role": ["TEST"], "colDisplayName": "displacement"}
					]
				}
			]
			}`))
	assert.NoError(t, err)

	actual, err := json.Unmarshal(res.Body.Bytes())
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}
