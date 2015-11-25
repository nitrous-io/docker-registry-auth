package main

type Authorizer interface {
	Authorize(ai *AuthorizationInfo) ([]string, error)
}

type AuthorizationInfo struct {
	Account string
	Service string
	Type    string
	Name    string
	Actions []string
}

type NitrousAuthorizer struct {
}

func (a *NitrousAuthorizer) Authorize(ai *AuthorizationInfo) ([]string, error) {
	return ai.Actions, nil
}
