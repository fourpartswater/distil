package datamart

import (
	"github.com/unchartedsoftware/distil/api/model"
	"github.com/unchartedsoftware/distil/api/rest"
)

// Storage accesses the underlying datamart instance.
type Storage struct {
	client *rest.Client
}

// NewMetadataStorage returns a constructor for a metadata storage.
func NewMetadataStorage(clientCtor rest.ClientCtor) model.MetadataStorageCtor {
	return func() (model.MetadataStorage, error) {
		return &Storage{
			client: clientCtor(),
		}, nil
	}
}
