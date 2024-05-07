package crud

// TODO: Get All Endpoint.

import "github.com/luikymagno/auth-server/internal/models"

type ScopeManager interface {
	Create(scope models.Scope) error
	Update(id string, scope models.Scope) error
	Get(id string) (models.Scope, error)
	Delete(id string) error
}

type GrantModelManager interface {
	Create(model models.GrantModel) error
	Update(id string, model models.GrantModel) error
	Get(id string) (models.GrantModel, error)
	Delete(id string) error
}

type ClientManager interface {
	Create(client models.Client) error
	Update(id string, client models.Client) error
	Get(id string) (models.Client, error)
	Delete(id string) error
}

type GrantSessionManager interface {
	CreateOrUpdate(token models.GrantSession) error
	Get(id string) (models.GrantSession, error)
	GetByTokenId(tokenId string) (models.GrantSession, error)
	GetByRefreshToken(refreshToken string) (models.GrantSession, error)
	Delete(id string) error
}

type AuthnSessionManager interface {
	CreateOrUpdate(session models.AuthnSession) error
	GetByCallbackId(callbackId string) (models.AuthnSession, error)
	GetByAuthorizationCode(authorizationCode string) (models.AuthnSession, error)
	GetByRequestUri(requestUri string) (models.AuthnSession, error)
	Delete(id string) error
}
