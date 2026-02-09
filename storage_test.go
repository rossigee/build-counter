package main

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestConfigMapStorage_StartBuild(t *testing.T) {
	client := fake.NewClientset()
	storage := &ConfigMapStorage{
		client:    client,
		namespace: "default",
		configMap: "build-counter",
	}

	// Test starting a build
	id, err := storage.StartBuild("test-project", "build-123")
	if err != nil {
		t.Errorf("StartBuild() error = %v", err)
		return
	}

	if id <= 0 {
		t.Errorf("StartBuild() returned invalid id = %v", id)
	}

	// Verify ConfigMap was created
	cm, err := client.CoreV1().ConfigMaps("default").Get(context.TODO(), "build-counter", metav1.GetOptions{})
	if err != nil {
		t.Errorf("ConfigMap was not created: %v", err)
		return
	}

	if _, exists := cm.Data["test-project"]; !exists {
		t.Errorf("Project data not found in ConfigMap")
	}
}

func TestConfigMapStorage_FinishBuild(t *testing.T) {
	client := fake.NewClientset()

	// Pre-create ConfigMap with build data
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-counter",
			Namespace: "default",
		},
		Data: map[string]string{
			"test-project": `{"name":"test-project","build_id":"build-123","started":"2023-01-01T00:00:00Z","id":1}`,
		},
	}
	_, err := client.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	storage := &ConfigMapStorage{
		client:    client,
		namespace: "default",
		configMap: "build-counter",
	}

	// Test finishing the build
	err = storage.FinishBuild("test-project", "build-123")
	if err != nil {
		t.Errorf("FinishBuild() error = %v", err)
		return
	}

	// Verify build was marked as finished
	cm, err = client.CoreV1().ConfigMaps("default").Get(context.TODO(), "build-counter", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
		return
	}

	data := cm.Data["test-project"]
	if !containsSubstring(data, "finished") {
		t.Errorf("Build was not marked as finished")
	}
}

func TestConfigMapStorage_HealthCheck(t *testing.T) {
	// Test with healthy client
	client := fake.NewClientset()
	storage := &ConfigMapStorage{
		client:    client,
		namespace: "default",
		configMap: "build-counter",
	}

	err := storage.HealthCheck()
	if err != nil {
		t.Errorf("HealthCheck() error = %v, want nil", err)
	}

	// Test with failing client
	client.PrependReactor("get", "configmaps", func(_ ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("Internal Server Error")
	})

	err = storage.HealthCheck()
	if err == nil {
		t.Errorf("HealthCheck() error = nil, want error for failing client")
	}
}

func TestConfigMapStorage_ListProjects(t *testing.T) {
	client := fake.NewClientset()

	// Pre-create ConfigMap with multiple projects
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-counter",
			Namespace: "default",
		},
		Data: map[string]string{
			"project1": `{"name":"project1","build_id":"build-1","started":"2023-01-01T00:00:00Z","finished":"2023-01-01T00:05:00Z","id":1}`,
			"project2": `{"name":"project2","build_id":"build-2","started":"2023-01-01T01:00:00Z","id":2}`,
		},
	}
	_, err := client.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	storage := &ConfigMapStorage{
		client:    client,
		namespace: "default",
		configMap: "build-counter",
	}

	projects, err := storage.ListProjects()
	if err != nil {
		t.Errorf("ListProjects() error = %v", err)
		return
	}

	if len(projects) != 2 {
		t.Errorf("ListProjects() returned %d projects, want 2", len(projects))
		return
	}

	// Verify project names
	names := make(map[string]bool)
	for _, p := range projects {
		names[p.Name] = true
	}

	if !names["project1"] || !names["project2"] {
		t.Errorf("ListProjects() missing expected projects")
	}
}

func TestConfigMapStorage_GetProjectBuilds(t *testing.T) {
	client := fake.NewClientset()

	// Pre-create ConfigMap with project data
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-counter",
			Namespace: "default",
		},
		Data: map[string]string{
			"test-project": `{"name":"test-project","build_id":"build-123","started":"2023-01-01T00:00:00Z","finished":"2023-01-01T00:05:00Z","id":1}`,
		},
	}
	_, err := client.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create test ConfigMap: %v", err)
	}

	storage := &ConfigMapStorage{
		client:    client,
		namespace: "default",
		configMap: "build-counter",
	}

	builds, err := storage.GetProjectBuilds("test-project")
	if err != nil {
		t.Errorf("GetProjectBuilds() error = %v", err)
		return
	}

	if len(builds) != 1 {
		t.Errorf("GetProjectBuilds() returned %d builds, want 1", len(builds))
		return
	}

	build := builds[0]
	if build.Name != "test-project" {
		t.Errorf("GetProjectBuilds() build name = %v, want test-project", build.Name)
	}

	if build.BuildID != "build-123" {
		t.Errorf("GetProjectBuilds() build ID = %v, want build-123", build.BuildID)
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test storage interface compliance
func TestStorageInterface(_ *testing.T) {
	// Test that ConfigMapStorage implements Storage interface
	var _ Storage = &ConfigMapStorage{}

	// Test that DatabaseStorage implements Storage interface
	var _ Storage = &DatabaseStorage{}
}

// Benchmark tests
func BenchmarkConfigMapStorage_StartBuild(b *testing.B) {
	client := fake.NewClientset()
	storage := &ConfigMapStorage{
		client:    client,
		namespace: "default",
		configMap: "build-counter",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = storage.StartBuild("bench-project", "build-"+time.Now().String())
	}
}

func BenchmarkConfigMapStorage_ListProjects(b *testing.B) {
	client := fake.NewClientset()

	// Pre-populate with test data
	data := make(map[string]string)
	for i := 0; i < 100; i++ {
		projectName := "project-" + time.Now().String()
		data[projectName] = `{"name":"` + projectName + `","build_id":"build-1","started":"2023-01-01T00:00:00Z","id":1}`
	}

	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "build-counter",
			Namespace: "default",
		},
		Data: data,
	}
	_, _ = client.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{})

	storage := &ConfigMapStorage{
		client:    client,
		namespace: "default",
		configMap: "build-counter",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = storage.ListProjects()
	}
}
