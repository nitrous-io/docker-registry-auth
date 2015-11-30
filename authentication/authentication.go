package authentication

import "database/sql"

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

type NitrousAuthenticator struct {
	DB *sql.DB
}

const dockerRegistryTokenType = "docker_registry" // Tokens in the database should be of this type.

func (na *NitrousAuthenticator) Authenticate(user string, password PasswordString) (bool, error) {
	var userID int64
	err := na.DB.QueryRow(`SELECT user_id FROM tokens WHERE
			type = $1
			AND token = $2
			AND user_id = (SELECT id FROM users WHERE username = $3)
			AND user_id IS NOT NULL`,
		dockerRegistryTokenType, string(password), user).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
