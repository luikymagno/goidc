package inmemory

import (
	"context"
	"errors"

	"github.com/luikyv/go-oidc/pkg/goidc"
)

type ClientManager struct {
	Clients map[string]*goidc.Client
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		Clients: make(map[string]*goidc.Client),
	}
}

func (manager *ClientManager) Save(
	_ context.Context,
	client *goidc.Client,
) error {
	manager.Clients[client.ID] = client
	return nil
}

func (manager *ClientManager) Get(
	_ context.Context,
	id string,
) (
	*goidc.Client,
	error,
) {
	client, exists := manager.Clients[id]
	if !exists {
		return nil, errors.New("entity not found")
	}

	return client, nil
}

func (manager *ClientManager) Delete(
	_ context.Context,
	id string,
) error {
	delete(manager.Clients, id)
	return nil
}
