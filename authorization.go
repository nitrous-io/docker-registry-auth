package main

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
)

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

type NitrousUserAuthorizer struct {
	DB *sql.DB
}

const (
	roleOwner   = "owner"
	roleAdmin   = "admin"
	roleMember  = "member"
	roleLimited = "limited"
)

var (
	roleToActions = map[string][]string{
		roleOwner:   []string{"pull", "push", "delete"},
		roleAdmin:   []string{"pull", "push", "delete"},
		roleMember:  []string{"pull"},
		roleLimited: []string{},
	}

	ErrAuthInvalidType = errors.New("invalid type - only the 'repository' type is supported")
	ErrAuthInvalidRepo = errors.New("invalid repository format")
)

func (na *NitrousUserAuthorizer) Authorize(ai *AuthorizationInfo) ([]string, error) {
	if ai.Type != "repository" {
		return nil, ErrAuthInvalidType
	}

	username := ai.Account
	repoName := ai.Name // This will not contain the repository tag since the Docker Registry does not send it.

	parts := strings.Split(repoName, "/")
	if len(parts) != 2 {
		return nil, ErrAuthInvalidRepo
	}
	orgSlug := parts[0]
	// snapshotSlug := parts[1] // TODO Used to check whether user is creator of repository.

	// Verify that user belongs to org and get their role.
	var role string
	err := na.DB.QueryRow(`SELECT role FROM users
		JOIN org_memberships ON users.id = org_memberships.user_id
		JOIN orgs ON org_memberships.org_id = orgs.id
		WHERE username = $1 AND orgs.slug = $2`, username, orgSlug).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return []string{}, nil
		}
		return nil, err
	}

	// TODO Check whether user is creator of the repository (aka snapshot) - grant full access if so.

	// Determine actions allowed depending on user's role in the org.
	allowedActions, ok := roleToActions[role]
	if ok {
		return intersect(allowedActions, ai.Actions), nil
	}

	return []string{}, nil
}

func intersect(a, b []string) []string {
	intersection := []string{}
	for _, s := range a {
		if contains(b, s) {
			intersection = append(intersection, s)
		}
	}
	sort.Strings(intersection)
	return intersection
}

func contains(slice []string, elem string) bool {
	for _, s := range slice {
		if s == elem {
			return true
		}
	}
	return false
}
