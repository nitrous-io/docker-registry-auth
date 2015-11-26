package main

import (
	"database/sql"
	"fmt"
	"os/user"

	"github.com/DATA-DOG/go-txdb"
)

func init() {
	u, err := user.Current()
	if err != nil {
		panic("unable to determine current user")
	}
	dbname := "docker_registry_auth_test"

	dns := fmt.Sprintf("host=/var/run/postgresql user=%s dbname=%s sslmode=disable", u.Username, dbname)
	txdb.Register("txdb", "postgres", dns)
}

func testDatabase() (*sql.DB, error) {
	db, err := sql.Open("txdb", "identifier")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	if err := setupDatabaseTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func setupDatabaseTables(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE users (
		id bigserial PRIMARY KEY NOT NULL,
		username character varying(30) NOT NULL);`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE tokens (
		id bigserial PRIMARY KEY NOT NULL,
		user_id bigint REFERENCES users(id) NOT NULL,
		token character varying(255) NOT NULL,
		type character varying(255) DEFAULT 'login' NOT NULL);`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE orgs (
		id bigserial PRIMARY KEY NOT NULL,
		slug character varying(255) NOT NULL);`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TABLE org_memberships (
		id bigserial PRIMARY KEY NOT NULL,
		org_id bigint REFERENCES orgs(id) NOT NULL,
		user_id bigint REFERENCES users(id) NOT NULL,
		role character varying(20) DEFAULT 'member' NOT NULL);`); err != nil {
		return err
	}
	return nil
}
