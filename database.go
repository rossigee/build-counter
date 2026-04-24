package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

const (
	maxOpenConns    = 25
	maxIdleConns    = 25
	connMaxLifetime = 5 * time.Minute
	timeout5s       = 5 * time.Second
	timeout10s      = 10 * time.Second
	timeout2s       = 2 * time.Second
)

// DatabaseStorage handles build tracking using PostgreSQL
type DatabaseStorage struct {
	db *sql.DB
}

// NewDatabaseStorage creates a new database storage instance with a persistent connection pool
func NewDatabaseStorage() (*DatabaseStorage, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), timeout5s)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &DatabaseStorage{db: db}, nil
}

// StartBuild records the start of a build
func (ds *DatabaseStorage) StartBuild(name, buildID string) (int, error) {
	var nextID int
	query := "INSERT INTO builds (name, build_id, started) VALUES ($1, $2, now()) RETURNING id;"

	ctx, cancel := context.WithTimeout(context.Background(), timeout5s)
	defer cancel()

	if err := ds.db.QueryRowContext(ctx, query, name, buildID).Scan(&nextID); err != nil {
		return 0, fmt.Errorf("error inserting new build record: %w", err)
	}

	return nextID, nil
}

// FinishBuild records the completion of a build
func (ds *DatabaseStorage) FinishBuild(name, buildID string) error {
	query := "UPDATE builds SET finished = NOW() WHERE name = $1 AND build_id = $2"

	ctx, cancel := context.WithTimeout(context.Background(), timeout5s)
	defer cancel()

	result, err := ds.db.ExecContext(ctx, query, name, buildID)
	if err != nil {
		return fmt.Errorf("error updating finish time for name %s: %w", name, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("no build found for name %s and build_id %s", name, buildID)
	}

	return nil
}

// HealthCheck verifies the database connection is healthy
func (ds *DatabaseStorage) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout2s)
	defer cancel()

	if err := ds.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// ListProjects returns a summary of all projects with their latest builds
func (ds *DatabaseStorage) ListProjects() ([]ProjectSummary, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout10s)
	defer cancel()

	query := `
		SELECT DISTINCT ON (name)
			name,
			id,
			build_id,
			started,
			finished,
			(SELECT COUNT(*) FROM builds b2 WHERE b2.name = builds.name) as build_count
		FROM builds
		ORDER BY name, started DESC
	`

	rows, err := ds.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []ProjectSummary
	for rows.Next() {
		var p ProjectSummary
		var build Build
		var finished sql.NullTime

		if err := rows.Scan(&build.Name, &build.ID, &build.BuildID, &build.Started, &finished, &p.BuildCount); err != nil {
			return nil, fmt.Errorf("error scanning project row: %w", err)
		}

		if finished.Valid {
			build.Finished = &finished.Time
			duration := int64(finished.Time.Sub(build.Started).Seconds())
			build.Duration = &duration
		}

		p.Name = build.Name
		p.LatestBuild = build
		projects = append(projects, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project rows: %w", err)
	}

	return projects, nil
}

// GetProjectBuilds returns all builds for a specific project
func (ds *DatabaseStorage) GetProjectBuilds(name string) ([]Build, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout10s)
	defer cancel()

	query := `
		SELECT id, name, build_id, started, finished
		FROM builds
		WHERE name = $1
		ORDER BY started DESC
	`

	rows, err := ds.db.QueryContext(ctx, query, name)
	if err != nil {
		return nil, fmt.Errorf("error querying builds for project %s: %w", name, err)
	}
	defer func() { _ = rows.Close() }()

	var builds []Build
	for rows.Next() {
		var build Build
		var finished sql.NullTime

		if err := rows.Scan(&build.ID, &build.Name, &build.BuildID, &build.Started, &finished); err != nil {
			return nil, fmt.Errorf("error scanning build row: %w", err)
		}

		if finished.Valid {
			build.Finished = &finished.Time
			duration := int64(finished.Time.Sub(build.Started).Seconds())
			build.Duration = &duration
		}

		builds = append(builds, build)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating build rows: %w", err)
	}

	return builds, nil
}
