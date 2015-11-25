package main

type Authenticator interface {
	Authenticate(user, password string) (bool, error)
}

type NitrousUserAuthenticator struct {
}

func (a *NitrousUserAuthenticator) Authenticate(user, password string) (bool, error) {
	return true, nil
}
