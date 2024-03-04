package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"cloud.google.com/go/cloudsqlconn/postgres/pgxv4"
	"github.com/abcxyz/pkg/logging"
)

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

	createDeferredDeepLinksTable := `CREATE TABLE IF NOT EXISTS deep_links (
		user_ip VARCHAR(100), 
		device_type VARCHAR(100),
		target VARCHAR(100),
		PRIMARY KEY (user_ip)
    );`
	_, err = db.Exec(createDeferredDeepLinksTable)
	if err != nil {
		log.Fatalf("unable to create table: %s", err)
	}

	// See https://github.com/go-sql-driver/mysql/issues/257#issuecomment-53886663.
	db.SetMaxIdleConns(0)
	db.SetMaxOpenConns(500)
	db.SetConnMaxLifetime(time.Minute)

	return db, cleanup
}

func updateDatabaseForDeferredDeepLinkQuery(ctx context.Context, db *sql.DB, req *DeferredDeepLinkQueryRequest) error {
	logger := logging.FromContext(ctx)
	queryStr := populateUpsertQueryForDeferredDeepLinkQuery(req)
	logger.InfoContext(ctx, "running db query", "queryStr", queryStr)
	if _, err := db.Exec(queryStr); err != nil {
		return fmt.Errorf("db.Exec failed: %v", err)
	}
	return nil
}

func populateUpsertQueryForDeferredDeepLinkQuery(req *DeferredDeepLinkQueryRequest) string {
	query := `INSERT INTO deep_links (user_ip, device_type, target) VALUES ('%s', '%s', '%s') ON CONFLICT (user_ip) DO UPDATE SET target = EXCLUDED.target, device_type = EXCLUDED.device_type;`
	return fmt.Sprintf(query, req.UserIP, req.DeviceType, req.Target)
}

func queryDatabaseForDeferredDeepLink(ctx context.Context, db *sql.DB, ip string) (string, error) {
	var target string
	query := `SELECT target FROM deep_links WHERE user_ip = $1;`

	err := db.QueryRowContext(ctx, query, ip).Scan(&target)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("No result for given IP")
			return "", nil
		}
		return "", fmt.Errorf("QueryRowContext failed: %v", err)
	}

	log.Println("Running db query", "queryStr", query, "IP", ip)
	return target, nil
}
