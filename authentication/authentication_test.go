package authentication

import (
	"testing"

	"github.com/nitrous-io/docker-registry-auth/test"
)

func TestAuthenticate(t *testing.T) {
	db, err := test.TestDatabase()
	if err != nil {
		t.Fatalf("failed to initialize test database, err: %v", err)
	}
	defer db.Close()

	// Let's insert some data we can test against.
	if _, err := db.Exec(`INSERT INTO users (id, username) VALUES
		(1, 'unconfirmed-user'),
		(2, 'aroonthampy'),
		(3, 'beehoon-kim')
	`); err != nil {
		t.Fatalf("error inserting test data, err: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO tokens (user_id, token, type) VALUES
		(2, 'manu4evar', 'login'),
		(3, 'correct-password', 'docker_registry')
	`); err != nil {
		t.Fatalf("error inserting test data, err: %v", err)
	}

	na := &NitrousUserAuthenticator{DB: db}

	testCases := []struct {
		username string
		password string
		expected bool
	}{
		{
			// User does not exist.
			username: "does-not-exist",
			password: "does-not-matter",
			expected: false,
		},
		{
			// User does not have any tokens.
			username: "unconfirmed-user",
			password: "does-not-matter",
			expected: false,
		},
		{
			// User does not have a 'docker_registry' token.
			username: "aroonthampy",
			password: "manu4evar",
			expected: false,
		},
		{
			// Token/password incorrect.
			username: "beehoon-kim",
			password: "some-other-password",
			expected: false,
		},
		{
			username: "beehoon-kim",
			password: "correct-password",
			expected: true,
		},
	}

	for _, tc := range testCases {
		authenticated, err := na.Authenticate(tc.username, PasswordString(tc.password))
		if err != nil {
			t.Fatalf("error authenticating user %q", tc.username)
		}
		if authenticated != tc.expected {
			if tc.expected {
				t.Fatalf("expected user %q to be authenticated, but wasn't", tc.username)
			} else {
				t.Fatalf("expected user %q to be not be authenticated, but was", tc.username)
			}
		}
	}
}
