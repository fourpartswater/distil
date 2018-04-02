package routes

import (
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"goji.io/pat"

	"github.com/unchartedsoftware/distil/api/model"
)

// ResultVariableExtremaHandler returns the extremas for a variable relative to the result dataset.
func ResultVariableExtremaHandler(pipelineCtor model.PipelineStorageCtor, dataCtor model.DataStorageCtor) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// extract route parameters
		index := pat.Param(r, "index")
		dataset := pat.Param(r, "dataset")
		variable := pat.Param(r, "variable")
		resultUUID, err := url.PathUnescape(pat.Param(r, "results-uuid"))
		if err != nil {
			handleError(w, errors.Wrap(err, "unable to unescape results uuid"))
			return
		}

		pipeline, err := pipelineCtor()
		if err != nil {
			handleError(w, err)
			return
		}

		data, err := dataCtor()
		if err != nil {
			handleError(w, err)
			return
		}

		// get the result URI. Error ignored to make it ES compatible.
		res, err := pipeline.FetchResultMetadataByUUID(resultUUID)
		if err != nil {
			handleError(w, err)
			return
		}

		extrema, err := data.FetchExtremaByURI(dataset, res.ResultURI, index, variable)
		if err != nil {
			handleError(w, err)
			return
		}

		// marshall data and sent the response back
		err = handleJSON(w, map[string]interface{}{
			"extrema": extrema,
		})
		if err != nil {
			handleError(w, errors.Wrap(err, "unable marshal result histogram into JSON"))
			return
		}
	}
}