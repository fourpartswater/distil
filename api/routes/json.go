package routes

import (
	"net/http"

	"github.com/uncharted-distil/distil/api/util/json"
)

func handleJSON(w http.ResponseWriter, data interface{}) error {
	// marshal data
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// send response
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
	return nil
}
