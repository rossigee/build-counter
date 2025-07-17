package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// DatabaseStorage handles build tracking using PostgreSQL
type DatabaseStorage struct {
	connStr string
}

// NewDatabaseStorage creates a new database storage instance
func NewDatabaseStorage() (*DatabaseStorage, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	return &DatabaseStorage{
		connStr: connStr,
	}, nil
}

// connectDatabase creates a database connection
func (ds *DatabaseStorage) connectDatabase() (*sql.DB, error) {
	db, err := sql.Open("postgres", ds.connStr)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// StartBuild records the start of a build
func (ds *DatabaseStorage) StartBuild(name, buildID string) (int, error) {
	var nextID int
	query := "INSERT INTO builds (name, build_id, started) VALUES ($1, $2, now()) RETURNING id;"
	
	db, err := ds.connectDatabase()
	if err != nil {
		return 0, fmt.Errorf("unable to connect to database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.QueryRowContext(ctx, query, name, buildID).Scan(&nextID)
	if err != nil {
		return 0, fmt.Errorf("error inserting new build record: %w", err)
	}

	return nextID, nil
}

// FinishBuild records the completion of a build
func (ds *DatabaseStorage) FinishBuild(name, buildID string) error {
	query := "UPDATE builds SET finished = NOW() WHERE name = $1 AND build_id = $2"
	
	db, err := ds.connectDatabase()
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = db.ExecContext(ctx, query, name, buildID)
	if err != nil {
		return fmt.Errorf("error updating finish time for name %s: %w", name, err)
	}

	return nil
}

// HealthCheck verifies the database connection is healthy
func (ds *DatabaseStorage) HealthCheck() error {
	db, err := ds.connectDatabase()
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// ListProjects returns a summary of all projects with their latest builds
func (ds *DatabaseStorage) ListProjects() ([]ProjectSummary, error) {
	db, err := ds.connectDatabase()
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying projects: %w", err)
	}
	defer rows.Close()

	var projects []ProjectSummary
	for rows.Next() {
		var p ProjectSummary
		var build Build
		var finished sql.NullTime
		
		err := rows.Scan(&build.Name, &build.ID, &build.BuildID, &build.Started, &finished, &p.BuildCount)
		if err != nil {
			return nil, fmt.Errorf("error scanning project row: %w", err)
		}
		
		if finished.Valid {
			build.Finished = &finished.Time
			duration := finished.Time.Sub(build.Started).Seconds()
			durationInt := int64(duration)
			build.Duration = &durationInt
		}
		
		p.Name = build.Name
		p.LatestBuild = build
		projects = append(projects, p)
	}

	return projects, nil
}

// GetProjectBuilds returns all builds for a specific project
func (ds *DatabaseStorage) GetProjectBuilds(name string) ([]Build, error) {
	db, err := ds.connectDatabase()
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := `
		SELECT id, name, build_id, started, finished
		FROM builds 
		WHERE name = $1
		ORDER BY started DESC
	`

	rows, err := db.QueryContext(ctx, query, name)
	if err != nil {
		return nil, fmt.Errorf("error querying builds for project %s: %w", name, err)
	}
	defer rows.Close()

	var builds []Build
	for rows.Next() {
		var build Build
		var finished sql.NullTime
		
		err := rows.Scan(&build.ID, &build.Name, &build.BuildID, &build.Started, &finished)
		if err != nil {
			return nil, fmt.Errorf("error scanning build row: %w", err)
		}
		
		if finished.Valid {
			build.Finished = &finished.Time
			duration := finished.Time.Sub(build.Started).Seconds()
			durationInt := int64(duration)
			build.Duration = &durationInt
		}
		
		builds = append(builds, build)
	}

	return builds, nil
}