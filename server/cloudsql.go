package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	// "cloud.google.com/go/cloudsqlconn"

	"cloud.google.com/go/cloudsqlconn"
	"cloud.google.com/go/cloudsqlconn/postgres/pgxv4"
)

const (
	insertQuery = `
`
)

type visit struct {
	device_id string
	pill      string
}

func getDB() (*sql.DB, func() error) {
	cleanup, err := pgxv4.RegisterDriver("cloudsql-postgres", cloudsqlconn.WithIAMAuthN())
	if err != nil {
		log.Fatalf("Error on pgxv4.RegisterDriver: %v", err)
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable", os.Getenv("INSTANCE_CONNECTION_NAME"), os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_NAME"))
	db, err := sql.Open("cloudsql-postgres", dsn)
	if err != nil {
		log.Fatalf("Error on sql.Open: %v", err)
	}

	createVisits := `CREATE TEMPORARY TABLE IF NOT EXISTS deferredVisits (
		device_ip VARCHAR(100), 
		pill VARCHAR(100),
		PRIMARY KEY (device_ip)
    );`
	_, err = db.Exec(createVisits)
	if err != nil {
		log.Fatalf("unable to create table: %s", err)
	}

	// See https://github.com/go-sql-driver/mysql/issues/257#issuecomment-53886663.
	db.SetMaxIdleConns(0)
	db.SetMaxOpenConns(500)
	db.SetConnMaxLifetime(time.Minute)

	return db, cleanup
}

// func connectWithConnector() (*sql.DB, error) {
// 	mustGetenv := func(k string) string {
// 		v := os.Getenv(k)
// 		if v == "" {
// 			log.Fatalf("Fatal Error in connect_connector.go: %s environment variable not set.", k)
// 		}
// 		return v
// 	}

// 	var (
// 		dbUser                 = mustGetenv("DB_USER")                  // e.g. 'my-db-user'
// 		dbPwd                  = mustGetenv("DB_PASS")                  // e.g. 'my-db-password'
// 		dbName                 = mustGetenv("DB_NAME")                  // e.g. 'my-database'
// 		instanceConnectionName = mustGetenv("INSTANCE_CONNECTION_NAME") // e.g. 'project:region:instance'
// 	)

// 	d, err := cloudsqlconn.NewDialer(context.Background())
// 	if err != nil {
// 		return nil, fmt.Errorf("cloudsqlconn.NewDialer: %w", err)
// 	}
// 	var opts []cloudsqlconn.DialOption
// 	mysql.RegisterDialContext("cloudsqlconn",
// 		func(ctx context.Context, addr string) (net.Conn, error) {
// 			return d.Dial(ctx, instanceConnectionName, opts...)
// 		})

// 	dbURI := fmt.Sprintf("%s:%s@cloudsqlconn(localhost:3306)/%s?parseTime=true",
// 		dbUser, dbPwd, dbName)

// 	dbPool, err := sql.Open("mysql", dbURI)
// 	if err != nil {
// 		return nil, fmt.Errorf("sql.Open: %w", err)
// 	}
// 	return dbPool, nil
// }

func updateDatabaseForDeferredAppLinkQuery(db *sql.DB, req *DeferredAppLinkQueryRequest) error {
	queryStr := populateUpsertQueryForDeferredAppLinkQuery(req)
	if _, err := db.Exec(queryStr); err != nil {
		return fmt.Errorf("db.Exec failed: %v", err)
	}
	return nil
}

func populateUpsertQueryForDeferredAppLinkQuery(req *DeferredAppLinkQueryRequest) string {
	values := []string{req.DeviceID, req.Pill}
	query := `INSERT INTO deferredVisits VALUES (%s) ON DUPLICATE KEY UPDATE pill = %s;`
	return fmt.Sprintf(query, strings.Join(values, ", "), req.Pill)
}
