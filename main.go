package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
)

// Version is set at build time
var version = "0.9.0"

// Build represents a build record
type Build struct {
	ID       int        `json:"id"`
	Name     string     `json:"name"`
	BuildID  string     `json:"build_id"`
	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`
	Duration *int64     `json:"duration_seconds,omitempty"`
}

// ProjectSummary represents the latest build for a project
type ProjectSummary struct {
	Name        string     `json:"name"`
	LatestBuild Build      `json:"latest_build"`
	BuildCount  int        `json:"build_count,omitempty"`
}

// Global storage interface
type Storage interface {
	StartBuild(name, buildID string) (int, error)
	FinishBuild(name, buildID string) error
	HealthCheck() error
	ListProjects() ([]ProjectSummary, error)
	GetProjectBuilds(name string) ([]Build, error)
}

// Global storage instance
var storage Storage

// Metrics tracking
var (
	startTime        = time.Now()
	requestsTotal    int64
	buildsStarted    int64
	buildsFinished   int64
	healthChecks     int64
	errorCount       int64
)

type Response struct {
	NextID int `json:"next_id"`
}

// Input validation patterns
var (
	namePattern    = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	buildIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
)

// validateInput validates and sanitizes input parameters
func validateInput(name, buildID string) error {
	if len(name) == 0 || len(name) > 255 {
		return fmt.Errorf("name must be between 1 and 255 characters")
	}
	if len(buildID) == 0 || len(buildID) > 255 {
		return fmt.Errorf("build_id must be between 1 and 255 characters")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("name contains invalid characters")
	}
	if !buildIDPattern.MatchString(buildID) {
		return fmt.Errorf("build_id contains invalid characters")
	}
	return nil
}

func startBuildHandler() http.HandlerFunc {
	log.Println("Initialising 'startBuildHandler' function...")

	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		_, span := startSpan(r.Context(), "start-build")
		defer span.End()

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		build_id := strings.TrimSpace(r.URL.Query().Get("build_id"))

		span.SetAttributes(
			attribute.String("build.name", name),
			attribute.String("build.id", build_id),
		)

		if err := validateInput(name, build_id); err != nil {
			log.Printf("Invalid input: %v", err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Invalid input parameters", http.StatusBadRequest)
			return
		}

		nextID, err := storage.StartBuild(name, build_id)
		if err != nil {
			log.Printf("Error starting build: %v", err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		atomic.AddInt64(&buildsStarted, 1)
		span.SetAttributes(attribute.Int("build.next_id", nextID))

		resp := Response{NextID: nextID}
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshaling JSON response: %v", err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResp)
	}
}

func finishBuildHandler() http.HandlerFunc {
	log.Println("Initialising 'finishBuildHandler' function...")

	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		_, span := startSpan(r.Context(), "finish-build")
		defer span.End()

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		build_id := strings.TrimSpace(r.URL.Query().Get("build_id"))

		span.SetAttributes(
			attribute.String("build.name", name),
			attribute.String("build.id", build_id),
		)

		if err := validateInput(name, build_id); err != nil {
			log.Printf("Invalid input: %v", err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Invalid input parameters", http.StatusBadRequest)
			return
		}

		err := storage.FinishBuild(name, build_id)
		if err != nil {
			log.Printf("Error finishing build: %v", err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		atomic.AddInt64(&buildsFinished, 1)
		w.WriteHeader(http.StatusCreated)
	}
}

func connectDatabase() (*sql.DB, error) {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	db, err := sql.Open("postgres", connStr)
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

// securityHeadersMiddleware adds security headers to responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// methodFilter ensures only POST requests are allowed
func methodFilter(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

// healthHandler provides a health check endpoint
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		atomic.AddInt64(&healthChecks, 1)
		_, span := startSpan(r.Context(), "health-check")
		defer span.End()

		if err := storage.HealthCheck(); err != nil {
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Storage health check failed"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// livenessHandler provides a liveness probe endpoint (/healthz)
func livenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple liveness check - just return OK if the service is running
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// readinessHandler provides a readiness probe endpoint (/readyz)
func readinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		atomic.AddInt64(&healthChecks, 1)
		_, span := startSpan(r.Context(), "readiness-check")
		defer span.End()

		// Readiness check - verify that storage is accessible
		if err := storage.HealthCheck(); err != nil {
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Storage not ready"))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// metricsHandler provides a Prometheus-style metrics endpoint
func metricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		
		uptime := time.Since(startTime).Seconds()
		
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		
		// Get current storage type
		storageType := "database"
		if lightweightMode {
			storageType = "configmap"
		}
		
		metrics := fmt.Sprintf(`# HELP build_counter_info Information about the build counter service
# TYPE build_counter_info gauge
build_counter_info{version="%s",storage_type="%s"} 1

# HELP build_counter_uptime_seconds Total uptime of the service in seconds
# TYPE build_counter_uptime_seconds gauge
build_counter_uptime_seconds %.2f

# HELP build_counter_requests_total Total number of HTTP requests
# TYPE build_counter_requests_total counter
build_counter_requests_total %d

# HELP build_counter_builds_started_total Total number of builds started
# TYPE build_counter_builds_started_total counter
build_counter_builds_started_total %d

# HELP build_counter_builds_finished_total Total number of builds finished
# TYPE build_counter_builds_finished_total counter
build_counter_builds_finished_total %d

# HELP build_counter_health_checks_total Total number of health checks
# TYPE build_counter_health_checks_total counter
build_counter_health_checks_total %d

# HELP build_counter_errors_total Total number of errors
# TYPE build_counter_errors_total counter
build_counter_errors_total %d

# HELP build_counter_memory_usage_bytes Memory usage in bytes
# TYPE build_counter_memory_usage_bytes gauge
build_counter_memory_usage_bytes %d

# HELP build_counter_goroutines Number of goroutines
# TYPE build_counter_goroutines gauge
build_counter_goroutines %d

# HELP process_start_time_seconds Start time of the process since unix epoch in seconds
# TYPE process_start_time_seconds gauge
process_start_time_seconds %.2f
`,
			version,
			storageType,
			uptime,
			atomic.LoadInt64(&requestsTotal),
			atomic.LoadInt64(&buildsStarted),
			atomic.LoadInt64(&buildsFinished),
			atomic.LoadInt64(&healthChecks),
			atomic.LoadInt64(&errorCount),
			m.Alloc,
			runtime.NumGoroutine(),
			float64(startTime.Unix()),
		)
		
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(metrics))
	}
}

// apiProjectsHandler provides REST API for listing projects
func apiProjectsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		_, span := startSpan(r.Context(), "api-projects")
		defer span.End()

		projects, err := storage.ListProjects()
		if err != nil {
			log.Printf("Error listing projects: %v", err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(projects)
	}
}

// apiProjectBuildsHandler provides REST API for listing builds for a project
func apiProjectBuildsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		_, span := startSpan(r.Context(), "api-project-builds")
		defer span.End()

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			http.Error(w, "Missing 'name' parameter", http.StatusBadRequest)
			return
		}

		span.SetAttributes(attribute.String("project.name", name))

		builds, err := storage.GetProjectBuilds(name)
		if err != nil {
			log.Printf("Error getting builds for project %s: %v", name, err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(builds)
	}
}

// homepageHandler provides the main HTML interface
func homepageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		_, span := startSpan(r.Context(), "homepage")
		defer span.End()

		projects, err := storage.ListProjects()
		if err != nil {
			log.Printf("Error listing projects: %v", err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		storageType := "Database (PostgreSQL)"
		if lightweightMode {
			storageType = "Lightweight (Kubernetes ConfigMap)"
		}

		html := generateHomepageHTML(projects, storageType)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

// projectBuildsHandler provides the project builds HTML interface
func projectBuildsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestsTotal, 1)
		_, span := startSpan(r.Context(), "project-builds")
		defer span.End()

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			http.Error(w, "Missing 'name' parameter", http.StatusBadRequest)
			return
		}

		span.SetAttributes(attribute.String("project.name", name))

		builds, err := storage.GetProjectBuilds(name)
		if err != nil {
			log.Printf("Error getting builds for project %s: %v", name, err)
			recordError(span, err)
			atomic.AddInt64(&errorCount, 1)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		html := generateProjectBuildsHTML(name, builds)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}
}

// generateHomepageHTML generates the main page HTML
func generateHomepageHTML(projects []ProjectSummary, storageType string) string {
	var projectRows strings.Builder
	
	for _, project := range projects {
		status := "Running"
		statusClass := "status-running"
		duration := "N/A"
		
		if project.LatestBuild.Finished != nil {
			status = "Completed"
			statusClass = "status-completed"
			if project.LatestBuild.Duration != nil {
				duration = fmt.Sprintf("%ds", *project.LatestBuild.Duration)
			}
		}
		
		// Only make clickable if not in lightweight mode
		if lightweightMode {
			projectRows.WriteString(fmt.Sprintf(`
				<tr>
					<td>%s</td>
					<td>%s</td>
					<td><span class="%s">%s</span></td>
					<td>%s</td>
					<td>%s</td>
					<td>%d</td>
				</tr>`,
				project.Name,
				project.LatestBuild.BuildID,
				statusClass,
				status,
				project.LatestBuild.Started.Format("2006-01-02 15:04:05"),
				duration,
				project.BuildCount,
			))
		} else {
			projectRows.WriteString(fmt.Sprintf(`
				<tr onclick="window.location.href='/project?name=%s'" style="cursor: pointer;">
					<td>%s</td>
					<td>%s</td>
					<td><span class="%s">%s</span></td>
					<td>%s</td>
					<td>%s</td>
					<td>%d</td>
				</tr>`,
				project.Name,
				project.Name,
				project.LatestBuild.BuildID,
				statusClass,
				status,
				project.LatestBuild.Started.Format("2006-01-02 15:04:05"),
				duration,
				project.BuildCount,
			))
		}
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Build Counter - %s</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; background-color: #0d1117; color: #c9d1d9; }
        .container { max-width: 1200px; margin: 20px auto; background-color: #161b22; padding: 24px; border-radius: 12px; border: 1px solid #30363d; }
        h1 { color: #f0f6fc; text-align: center; margin-bottom: 24px; }
        .info { background-color: #0d1117; padding: 16px; border-radius: 8px; margin-bottom: 24px; border: 1px solid #30363d; }
        .info strong { color: #58a6ff; }
        table { width: 100%%; border-collapse: collapse; margin-top: 20px; }
        th, td { padding: 12px 16px; text-align: left; border-bottom: 1px solid #30363d; }
        th { background-color: #0d1117; font-weight: 600; color: #f0f6fc; }
        tr:hover { background-color: #1c2128; }
        .status-running { color: #f9826c; font-weight: 600; }
        .status-completed { color: #3fb950; font-weight: 600; }
        .footer { margin-top: 32px; text-align: center; color: #8b949e; font-size: 14px; }
        .api-links { margin-top: 24px; padding-top: 24px; border-top: 1px solid #30363d; }
        .api-links a { margin-right: 20px; color: #58a6ff; text-decoration: none; font-weight: 500; }
        .api-links a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Build Counter Dashboard</h1>
        <div class="info">
            <strong>Storage Mode:</strong> %s<br>
            <strong>Version:</strong> %s<br>
            <strong>Projects:</strong> %d
        </div>
        
        <table>
            <thead>
                <tr>
                    <th>Project</th>
                    <th>Latest Build ID</th>
                    <th>Status</th>
                    <th>Started</th>
                    <th>Duration</th>
                    <th>Total Builds</th>
                </tr>
            </thead>
            <tbody>
                %s
            </tbody>
        </table>
        
        <div class="api-links">
            <strong>API Endpoints:</strong>
            <a href="/api/projects">JSON Projects</a>
            <a href="/metrics">Metrics</a>
            <a href="/health">Health</a>
        </div>
        
        <div class="footer">
            Build Counter v%s | %s mode%s
        </div>
    </div>
</body>
</html>`,
		storageType,
		storageType,
		version,
		len(projects),
		projectRows.String(),
		version,
		storageType,
		func() string {
			if lightweightMode {
				return ""
			}
			return " | Click rows to view build history"
		}(),
	)
}

// generateProjectBuildsHTML generates the project builds page HTML
func generateProjectBuildsHTML(projectName string, builds []Build) string {
	var buildRows strings.Builder
	
	for _, build := range builds {
		status := "Running"
		statusClass := "status-running"
		duration := "N/A"
		finishedTime := "N/A"
		
		if build.Finished != nil {
			status = "Completed"
			statusClass = "status-completed"
			finishedTime = build.Finished.Format("2006-01-02 15:04:05")
			if build.Duration != nil {
				duration = fmt.Sprintf("%ds", *build.Duration)
			}
		}
		
		buildRows.WriteString(fmt.Sprintf(`
			<tr>
				<td>%d</td>
				<td>%s</td>
				<td><span class="%s">%s</span></td>
				<td>%s</td>
				<td>%s</td>
				<td>%s</td>
			</tr>`,
			build.ID,
			build.BuildID,
			statusClass,
			status,
			build.Started.Format("2006-01-02 15:04:05"),
			finishedTime,
			duration,
		))
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Build Counter - %s</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; background-color: #0d1117; color: #c9d1d9; }
        .container { max-width: 1200px; margin: 20px auto; background-color: #161b22; padding: 24px; border-radius: 12px; border: 1px solid #30363d; }
        h1 { color: #f0f6fc; text-align: center; margin-bottom: 24px; }
        .breadcrumb { margin-bottom: 20px; }
        .breadcrumb a { color: #58a6ff; text-decoration: none; font-weight: 500; }
        .breadcrumb a:hover { text-decoration: underline; }
        table { width: 100%%; border-collapse: collapse; margin-top: 20px; }
        th, td { padding: 12px 16px; text-align: left; border-bottom: 1px solid #30363d; }
        th { background-color: #0d1117; font-weight: 600; color: #f0f6fc; }
        tr:hover { background-color: #1c2128; }
        .status-running { color: #f9826c; font-weight: 600; }
        .status-completed { color: #3fb950; font-weight: 600; }
        .footer { margin-top: 32px; text-align: center; color: #8b949e; font-size: 14px; }
        .api-links { margin-top: 24px; padding-top: 24px; border-top: 1px solid #30363d; }
        .api-links a { margin-right: 20px; color: #58a6ff; text-decoration: none; font-weight: 500; }
        .api-links a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <div class="breadcrumb">
            <a href="/">← Back to Dashboard</a>
        </div>
        <h1>Build History: %s</h1>
        
        <table>
            <thead>
                <tr>
                    <th>Build ID</th>
                    <th>Build Name</th>
                    <th>Status</th>
                    <th>Started</th>
                    <th>Finished</th>
                    <th>Duration</th>
                </tr>
            </thead>
            <tbody>
                %s
            </tbody>
        </table>
        
        <div class="api-links">
            <strong>API Endpoints:</strong>
            <a href="/api/projects/%s">JSON Builds</a>
            <a href="/api/projects">All Projects</a>
        </div>
        
        <div class="footer">
            Build Counter v%s | %d builds shown
        </div>
    </div>
</body>
</html>`,
		projectName,
		projectName,
		buildRows.String(),
		projectName,
		version,
		len(builds),
	)
}

// printHelp displays usage information
func printHelp() {
	fmt.Printf(`build-counter version %s

A simple HTTP service for tracking build start and finish times.

Usage:
  build-counter [options]

Options:
  --version          Show version information
  --help             Show this help message
  --lightweight      Use Kubernetes ConfigMap storage instead of database
  --health-check     Check if service is healthy (for Docker healthcheck)

Environment Variables:
  DATABASE_URL       PostgreSQL connection string (required for default mode)
  NAMESPACE          Kubernetes namespace for ConfigMap (default: default)
  CONFIGMAP_NAME     Name of ConfigMap to use (default: build-counter)

Examples:
  # Start with database (default)
  build-counter

  # Start with ConfigMap storage
  build-counter --lightweight

  # Check version
  build-counter --version

For more information, see: https://github.com/rossigee/build-counter
`, version)
}

var lightweightMode bool

func main() {
	// Handle command line flags
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Printf("build-counter version %s\n", version)
			return
		case "--help", "-h":
			printHelp()
			return
		case "--lightweight":
			lightweightMode = true
		case "--health-check":
			resp, err := http.Get("http://localhost:8080/health")
			if err != nil || resp.StatusCode != 200 {
				os.Exit(1)
			}
			os.Exit(0)
		default:
			fmt.Printf("Unknown option: %s\n", os.Args[1])
			fmt.Println("Use --help for usage information")
			os.Exit(1)
		}
	}

	// Initialize tracing
	tracingCleanup, err := initTracing()
	if err != nil {
		log.Printf("Failed to initialize tracing: %v", err)
		// Continue without tracing
	}
	defer tracingCleanup()

	// Initialize storage
	if lightweightMode {
		storage, err = NewConfigMapStorage()
		if err != nil {
			log.Fatalf("Failed to initialize ConfigMap storage: %v", err)
		}
		log.Println("Using Kubernetes ConfigMap storage")
	} else {
		storage, err = NewDatabaseStorage()
		if err != nil {
			log.Fatalf("Failed to initialize database storage: %v", err)
		}
		log.Println("Using PostgreSQL database storage")
	}

	// Set up routes
	mux := http.NewServeMux()
	
	// Build tracking endpoints
	mux.HandleFunc("/start", methodFilter(startBuildHandler()))
	mux.HandleFunc("/finish", methodFilter(finishBuildHandler()))
	
	// Health check endpoints
	mux.HandleFunc("/health", healthHandler())
	mux.HandleFunc("/healthz", livenessHandler())
	mux.HandleFunc("/readyz", readinessHandler())
	
	// Metrics endpoint
	mux.HandleFunc("/metrics", metricsHandler())
	
	// REST API endpoints
	mux.HandleFunc("/api/projects", apiProjectsHandler())
	mux.HandleFunc("/api/projects/", apiProjectBuildsHandler())
	
	// Web interface endpoints
	mux.HandleFunc("/", homepageHandler())
	mux.HandleFunc("/project", projectBuildsHandler())

	// Add OpenTelemetry HTTP instrumentation and security headers
	handler := securityHeadersMiddleware(otelhttp.NewHandler(mux, "build-counter"))

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	storageTypeStr := "database"
	if lightweightMode {
		storageTypeStr = "configmap"
	}
	
	fmt.Printf("Starting build-counter version %s on port 8080...\n", version)
	fmt.Printf("Storage: %s\n", storageTypeStr)
	fmt.Printf("Web interface: http://localhost:8080/\n")
	log.Fatal(server.ListenAndServe())
}
