package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateInput(t *testing.T) {
	tests := []struct {
		name     string
		buildID  string
		wantErr  bool
		errMsg   string
	}{
		{"valid-name", "build-123", false, ""},
		{"valid_name", "build.123", false, ""},
		{"", "build-123", true, "name must be between 1 and 255 characters"},
		{"valid-name", "", true, "build_id must be between 1 and 255 characters"},
		{"invalid name!", "build-123", true, "name contains invalid characters"},
		{"valid-name", "build@123", true, "build_id contains invalid characters"},
		{string(make([]byte, 256)), "build-123", true, "name must be between 1 and 255 characters"},
		{"valid-name", string(make([]byte, 256)), true, "build_id must be between 1 and 255 characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.buildID, func(t *testing.T) {
			err := validateInput(tt.name, tt.buildID)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("validateInput() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestMethodFilter(t *testing.T) {
	handler := methodFilter(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		method     string
		wantStatus int
	}{
		{"POST", http.StatusOK},
		{"GET", http.StatusMethodNotAllowed},
		{"PUT", http.StatusMethodNotAllowed},
		{"DELETE", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			rr := httptest.NewRecorder()

			handler(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("methodFilter() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":   "nosniff",
		"X-Frame-Options":          "DENY",
		"X-XSS-Protection":         "1; mode=block",
		"Referrer-Policy":          "strict-origin-when-cross-origin",
	}

	for header, expected := range expectedHeaders {
		if got := rr.Header().Get(header); got != expected {
			t.Errorf("securityHeadersMiddleware() header %s = %v, want %v", header, got, expected)
		}
	}
}

func TestHealthHandler(t *testing.T) {
	// Mock storage for testing
	originalStorage := storage
	defer func() { storage = originalStorage }()
	
	// Create a mock storage that always fails
	mockStorage := &MockStorage{}
	storage = mockStorage

	handler := healthHandler()

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	// Should fail with mock storage
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("healthHandler() status = %v, want %v", rr.Code, http.StatusServiceUnavailable)
	}
}

// MockStorage for testing
type MockStorage struct{}

func (m *MockStorage) StartBuild(name, buildID string) (int, error) {
	return 0, fmt.Errorf("mock error")
}

func (m *MockStorage) FinishBuild(name, buildID string) error {
	return fmt.Errorf("mock error")
}

func (m *MockStorage) HealthCheck() error {
	return fmt.Errorf("mock storage not available")
}

func (m *MockStorage) ListProjects() ([]ProjectSummary, error) {
	return nil, fmt.Errorf("mock error")
}

func (m *MockStorage) GetProjectBuilds(name string) ([]Build, error) {
	return nil, fmt.Errorf("mock error")
}

func TestStartBuildHandlerValidation(t *testing.T) {
	handler := startBuildHandler()

	tests := []struct {
		name       string
		url        string
		method     string
		wantStatus int
	}{
		{"missing name", "/start?build_id=123", "POST", http.StatusBadRequest},
		{"missing build_id", "/start?name=test", "POST", http.StatusBadRequest},
		{"invalid name", "/start?name=test!&build_id=123", "POST", http.StatusBadRequest},
		{"invalid build_id", "/start?name=test&build_id=123@", "POST", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			rr := httptest.NewRecorder()

			handler(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("startBuildHandler() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestFinishBuildHandlerValidation(t *testing.T) {
	handler := finishBuildHandler()

	tests := []struct {
		name       string
		url        string
		method     string
		wantStatus int
	}{
		{"missing name", "/finish?build_id=123", "POST", http.StatusBadRequest},
		{"missing build_id", "/finish?name=test", "POST", http.StatusBadRequest},
		{"invalid name", "/finish?name=test!&build_id=123", "POST", http.StatusBadRequest},
		{"invalid build_id", "/finish?name=test&build_id=123@", "POST", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			rr := httptest.NewRecorder()

			handler(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("finishBuildHandler() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}