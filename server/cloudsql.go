package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/go-sql-driver/mysql"
)

const (
	insertQuery = `
CREATE TEMPORARY TABLE IF NOT EXISTS %s (
	device_ip VARCHAR(100), 
	pill VARCHAR(20),
	PRIMARY KEY (device_ip)
);
INSERT INTO %s VALUES (%s) ON DUPLICATE KEY UPDATE pill = %s;
`
)

func connectWithConnector() (*sql.DB, error) {
	mustGetenv := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			log.Fatalf("Fatal Error in connect_connector.go: %s environment variable not set.", k)
		}
		return v
	}

	var (
		dbUser                 = mustGetenv("DB_USER")                  // e.g. 'my-db-user'
		dbPwd                  = mustGetenv("DB_PASS")                  // e.g. 'my-db-password'
		dbName                 = mustGetenv("DB_NAME")                  // e.g. 'my-database'
		instanceConnectionName = mustGetenv("INSTANCE_CONNECTION_NAME") // e.g. 'project:region:instance'
	)

	d, err := cloudsqlconn.NewDialer(context.Background())
	if err != nil {
		return nil, fmt.Errorf("cloudsqlconn.NewDialer: %w", err)
	}
	var opts []cloudsqlconn.DialOption
	mysql.RegisterDialContext("cloudsqlconn",
		func(ctx context.Context, addr string) (net.Conn, error) {
			return d.Dial(ctx, instanceConnectionName, opts...)
		})

	dbURI := fmt.Sprintf("%s:%s@cloudsqlconn(localhost:3306)/%s?parseTime=true",
		dbUser, dbPwd, dbName)

	dbPool, err := sql.Open("mysql", dbURI)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	return dbPool, nil
}

func updateDatabaseForDeferredLinks(db *sql.DB, table string, req *DeferQueryRequest) error {
	queryStr := populateUpsertQueryForDeferredLinks(table, req)
	if _, err := db.Exec(queryStr); err != nil {
		return fmt.Errorf("db.Exec failed: %v", err)
	}
	return nil
}

func populateUpsertQueryForDeferredLinks(table string, req *DeferQueryRequest) string {
	values := []string{req.DeviceID, req.Pill}
	return fmt.Sprintf(insertQuery, table, table, strings.Join(values, ", "), req.Pill)
}
