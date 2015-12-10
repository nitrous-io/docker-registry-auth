package authorization

import (
	"reflect"
	"testing"

	"github.com/nitrous-io/docker-registry-auth/test"
)

func TestAuthorize(t *testing.T) {
	db, err := test.TestDatabase()
	if err != nil {
		t.Fatalf("failed to initialize test database, err: %v", err)
	}
	defer db.Close()

	// Let's insert some data we can test against.
	if _, err := db.Exec(`INSERT INTO users (id, username) VALUES
		(1, 'individual-user'),
		(2, 'limited-user'),
		(3, 'member-user'),
		(4, 'admin-user'),
		(5, 'owner-user'),
		(6, 'other-org-user')
	`); err != nil {
		t.Fatalf("error inserting test data, err: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO orgs (id, slug) VALUES
		(1, 'org1'),
		(2, 'org2')
	`); err != nil {
		t.Fatalf("error inserting test data, err: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO org_memberships (org_id, user_id, role) VALUES
		(1, 2, '` + roleLimited + `'),
		(1, 3, '` + roleMember + `'),
		(1, 4, '` + roleAdmin + `'),
		(1, 5, '` + roleOwner + `'),
		(2, 6, '` + roleOwner + `')
	`); err != nil {
		t.Fatalf("error inserting test data, err: %v", err)
	}

	na := &NitrousAuthorizer{DB: db}

	testCases := []struct {
		ai          *AuthorizationInfo
		expected    []string
		expectedErr error
	}{
		{
			// Type is not "repository".
			ai: &AuthorizationInfo{
				Account: "member-user",
				Type:    "login",
				Name:    "org1/busybox",
				Actions: []string{"pull"},
			},
			expectedErr: ErrAuthInvalidType,
		},
		{
			// Invalid repository (name) format.
			ai: &AuthorizationInfo{
				Account: "individual-user",
				Type:    "repository",
				Name:    "busybox",
				Actions: []string{"push", "pull"},
			},
			expectedErr: ErrAuthInvalidRepo,
		},
		{
			// User does not belong to an org.
			ai: &AuthorizationInfo{
				Account: "individual-user",
				Type:    "repository",
				Name:    "org1/busybox",
				Actions: []string{"pull", "push"},
			},
			expected: []string{},
		},
		{
			// User tries to access repo from an org he's not a member of.
			ai: &AuthorizationInfo{
				Account: "other-org-user",
				Type:    "repository",
				Name:    "org1/busybox",
				Actions: []string{"pull", "push"},
			},
			expected: []string{},
		},
		{
			// Limited users should have no access.
			ai: &AuthorizationInfo{
				Account: "limited-user",
				Type:    "repository",
				Name:    "org1/busybox",
				Actions: []string{"pull", "push"},
			},
			expected: []string{},
		},
		{
			// Member users should have push and pull access.
			ai: &AuthorizationInfo{
				Account: "member-user",
				Type:    "repository",
				Name:    "org1/busybox",
				Actions: []string{"pull", "push"},
			},
			expected: []string{"pull", "push"},
		},
		{
			// Admin users should have pull, push and delete access.
			ai: &AuthorizationInfo{
				Account: "admin-user",
				Type:    "repository",
				Name:    "org1/busybox",
				Actions: []string{"pull", "push", "delete"},
			},
			expected: []string{"delete", "pull", "push"},
		},
		{
			// Owner users should have pull, push and delete access.
			ai: &AuthorizationInfo{
				Account: "owner-user",
				Type:    "repository",
				Name:    "org1/busybox",
				Actions: []string{"pull", "push", "delete"},
			},
			expected: []string{"delete", "pull", "push"},
		},
		{
			// Only return intersection of requested and authorized actions.
			ai: &AuthorizationInfo{
				Account: "owner-user",
				Type:    "repository",
				Name:    "org1/busybox",
				Actions: []string{"pull"},
			},
			expected: []string{"pull"},
		},
	}

	for _, tc := range testCases {
		authorizedActions, err := na.Authorize(tc.ai)

		if tc.expectedErr != nil && tc.expectedErr != err {
			t.Fatalf("expected Authorize(%+v) to return error %v but got %v", tc.ai, tc.expectedErr, err)
		}

		if tc.expected != nil && !reflect.DeepEqual(tc.expected, authorizedActions) {
			t.Fatalf("expected Authorize(%+v) to return %v but got %v", tc.ai, tc.expected, authorizedActions)
		}
	}
}

func TestIntersect(t *testing.T) {
	testCases := []struct {
		a        []string
		b        []string
		expected []string
	}{
		{
			a:        []string{},
			b:        []string{},
			expected: []string{},
		},
		{
			a:        []string{"pull"},
			b:        []string{},
			expected: []string{},
		},
		{
			a:        []string{},
			b:        []string{"pull"},
			expected: []string{},
		},
		{
			a:        []string{"pull"},
			b:        []string{"push"},
			expected: []string{},
		},
		{
			a:        []string{"pull"},
			b:        []string{"pull"},
			expected: []string{"pull"},
		},
		{
			a:        []string{"pull", "push"},
			b:        []string{"push"},
			expected: []string{"push"},
		},
		{
			a:        []string{"push", "pull", "delete"},
			b:        []string{"pull", "push"},
			expected: []string{"pull", "push"},
		},
	}

	for _, tc := range testCases {
		result := intersect(tc.a, tc.b)
		if !reflect.DeepEqual(result, tc.expected) {
			t.Fatalf("expected intersection of %v and %v to be %v, but got %v", tc.a, tc.b, tc.expected, result)
		}
	}
}

func TestContains(t *testing.T) {
	testCases := []struct {
		haystack []string
		needle   string
		expected bool
	}{
		{
			haystack: []string{},
			needle:   "pull",
			expected: false,
		},
		{
			haystack: []string{"pull", "push", "delete"},
			needle:   "kill",
			expected: false,
		},
		{
			haystack: []string{"pull", "push"},
			needle:   "delete",
			expected: false,
		},
		{
			haystack: []string{"pull", "push", "delete"},
			needle:   "pull",
			expected: true,
		},
	}

	for _, tc := range testCases {
		result := contains(tc.haystack, tc.needle)
		if result != tc.expected {
			if tc.expected {
				t.Fatalf("expected %v to contain %q, but it doesn't", tc.haystack, tc.needle)
			} else {
				t.Fatalf("expected %v not to contain %q, but it does", tc.haystack, tc.needle)
			}
		}
	}
}
