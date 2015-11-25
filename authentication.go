package main

type Authenticator interface {
	Authenticate(user string, password PasswordString) (bool, error)
}

type PasswordString string

func (s PasswordString) String() string {
	if len(s) == 0 {
		return ""
	}
	return "*** (REDACTED)"
}

type NitrousUserAuthenticator struct {
}

func (a *NitrousUserAuthenticator) Authenticate(user string, password PasswordString) (bool, error) {

	return true, nil
}
