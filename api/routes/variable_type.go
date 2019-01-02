package routes

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"goji.io/pat"

	api "github.com/unchartedsoftware/distil/api/model"
)

// VariableTypeHandler generates a route handler that facilitates the update
// of a variable type.
func VariableTypeHandler(storageCtor api.DataStorageCtor, metaCtor api.MetadataStorageCtor) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := getPostParameters(r)
		if err != nil {
			handleError(w, errors.Wrap(err, "Unable to parse post parameters"))
			return
		}
		field := params["field"].(string)
		typ := params["type"].(string)
		dataset := pat.Param(r, "dataset")

		// get clients
		storage, err := storageCtor()
		if err != nil {
			handleError(w, err)
			return
		}
		meta, err := metaCtor()
		if err != nil {
			handleError(w, err)
			return
		}

		// update the variable type in the storage
		err = storage.SetDataType(dataset, field, typ)
		if err != nil {
			handleError(w, errors.Wrap(err, "unable to update the data type in storage"))
			return
		}

		// update the variable type in the metadata
		err = meta.SetDataType(dataset, field, typ)
		if err != nil {
			handleError(w, errors.Wrap(err, "unable to update the data type in metadata"))
			return
		}

		// TODO: fix this, this shouldn't be necessary
		time.Sleep(time.Second)

		// marshall data
		err = handleJSON(w, map[string]interface{}{
			"success": true,
		})
		if err != nil {
			handleError(w, errors.Wrap(err, "unable marshal response into JSON"))
			return
		}
	}
}

func getPostParameters(r *http.Request) (map[string]interface{}, error) {
	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse POST request")
	}

	params := make(map[string]interface{})
	err = json.Unmarshal(body, &params)

	return params, err
}