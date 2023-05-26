module github.com/TravisS25/cdbm

go 1.15

replace github.com/TravisS25/webutil => /home/travis/go/src/github.com/TravisS25/webutil

replace github.com/jmoiron/sqlx => /home/travis/go/src/github.com/jmoiron/sqlx

require (
	github.com/DATA-DOG/go-sqlmock v1.5.0
	github.com/TravisS25/webutil v0.0.0-00010101000000-000000000000
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/jmoiron/sqlx v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.0
)
