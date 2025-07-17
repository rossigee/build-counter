package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// BuildInfo represents the structure stored in ConfigMap
type BuildInfo struct {
	Name      string    `json:"name"`
	BuildID   string    `json:"build_id"`
	Started   time.Time `json:"started"`
	Finished  *time.Time `json:"finished,omitempty"`
	ID        int       `json:"id"`
}

// ConfigMapStorage handles build tracking using Kubernetes ConfigMaps
type ConfigMapStorage struct {
	client    kubernetes.Interface
	namespace string
	configMap string
}

// NewConfigMapStorage creates a new ConfigMap storage instance
func NewConfigMapStorage() (*ConfigMapStorage, error) {
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	configMapName := os.Getenv("CONFIGMAP_NAME")
	if configMapName == "" {
		configMapName = "build-counter"
	}

	// Try in-cluster config first, then fallback to kubeconfig
	config, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &ConfigMapStorage{
		client:    clientset,
		namespace: namespace,
		configMap: configMapName,
	}, nil
}

// StartBuild records the start of a build
func (cms *ConfigMapStorage) StartBuild(name, buildID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get or create the ConfigMap
	cm, err := cms.client.CoreV1().ConfigMaps(cms.namespace).Get(ctx, cms.configMap, metav1.GetOptions{})
	if err != nil {
		// Create ConfigMap if it doesn't exist
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cms.configMap,
				Namespace: cms.namespace,
			},
			Data: make(map[string]string),
		}
		cm, err = cms.client.CoreV1().ConfigMaps(cms.namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return 0, fmt.Errorf("failed to create ConfigMap: %w", err)
		}
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	// Generate a simple ID (timestamp-based)
	buildInfo := BuildInfo{
		Name:     name,
		BuildID:  buildID,
		Started:  time.Now(),
		ID:       int(time.Now().Unix()),
	}

	data, err := json.Marshal(buildInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal build info: %w", err)
	}

	cm.Data[name] = string(data)

	// Update the ConfigMap
	_, err = cms.client.CoreV1().ConfigMaps(cms.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return buildInfo.ID, nil
}

// FinishBuild records the completion of a build
func (cms *ConfigMapStorage) FinishBuild(name, buildID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the ConfigMap
	cm, err := cms.client.CoreV1().ConfigMaps(cms.namespace).Get(ctx, cms.configMap, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	if cm.Data == nil {
		return fmt.Errorf("no build data found for name: %s", name)
	}

	// Get existing build info
	data, exists := cm.Data[name]
	if !exists {
		return fmt.Errorf("no build found for name: %s", name)
	}

	var buildInfo BuildInfo
	if err := json.Unmarshal([]byte(data), &buildInfo); err != nil {
		return fmt.Errorf("failed to unmarshal build info: %w", err)
	}

	// Verify build_id matches
	if buildInfo.BuildID != buildID {
		return fmt.Errorf("build_id mismatch: expected %s, got %s", buildInfo.BuildID, buildID)
	}

	// Update finish time
	now := time.Now()
	buildInfo.Finished = &now

	// Marshal back to JSON
	updatedData, err := json.Marshal(buildInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal updated build info: %w", err)
	}

	cm.Data[name] = string(updatedData)

	// Update the ConfigMap
	_, err = cms.client.CoreV1().ConfigMaps(cms.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return nil
}

// HealthCheck verifies the ConfigMap storage is accessible
func (cms *ConfigMapStorage) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := cms.client.CoreV1().ConfigMaps(cms.namespace).Get(ctx, cms.configMap, metav1.GetOptions{})
	if err != nil {
		// Try to create it if it doesn't exist
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cms.configMap,
				Namespace: cms.namespace,
			},
			Data: make(map[string]string),
		}
		_, err = cms.client.CoreV1().ConfigMaps(cms.namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to access or create ConfigMap: %w", err)
		}
	}
	return nil
}

// GetBuildInfo retrieves current build information for a name
func (cms *ConfigMapStorage) GetBuildInfo(name string) (*BuildInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cm, err := cms.client.CoreV1().ConfigMaps(cms.namespace).Get(ctx, cms.configMap, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	if cm.Data == nil {
		return nil, fmt.Errorf("no build data found")
	}

	data, exists := cm.Data[name]
	if !exists {
		return nil, fmt.Errorf("no build found for name: %s", name)
	}

	var buildInfo BuildInfo
	if err := json.Unmarshal([]byte(data), &buildInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal build info: %w", err)
	}

	return &buildInfo, nil
}

// ListBuilds returns all current build information
func (cms *ConfigMapStorage) ListBuilds() (map[string]BuildInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cm, err := cms.client.CoreV1().ConfigMaps(cms.namespace).Get(ctx, cms.configMap, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	builds := make(map[string]BuildInfo)
	if cm.Data == nil {
		return builds, nil
	}

	for name, data := range cm.Data {
		var buildInfo BuildInfo
		if err := json.Unmarshal([]byte(data), &buildInfo); err != nil {
			log.Printf("Error unmarshaling build info for %s: %v", name, err)
			continue
		}
		builds[name] = buildInfo
	}

	return builds, nil
}

// ListProjects returns a summary of all projects with their latest builds (ConfigMap mode)
func (cms *ConfigMapStorage) ListProjects() ([]ProjectSummary, error) {
	builds, err := cms.ListBuilds()
	if err != nil {
		return nil, fmt.Errorf("failed to list builds: %w", err)
	}

	var projects []ProjectSummary
	for name, buildInfo := range builds {
		build := Build{
			ID:      buildInfo.ID,
			Name:    buildInfo.Name,
			BuildID: buildInfo.BuildID,
			Started: buildInfo.Started,
		}
		
		if buildInfo.Finished != nil {
			build.Finished = buildInfo.Finished
			duration := buildInfo.Finished.Sub(buildInfo.Started).Seconds()
			durationInt := int64(duration)
			build.Duration = &durationInt
		}

		projects = append(projects, ProjectSummary{
			Name:        name,
			LatestBuild: build,
			BuildCount:  1, // ConfigMap only stores latest build
		})
	}

	return projects, nil
}

// GetProjectBuilds returns all builds for a specific project (ConfigMap mode - only latest)
func (cms *ConfigMapStorage) GetProjectBuilds(name string) ([]Build, error) {
	buildInfo, err := cms.GetBuildInfo(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get build info for %s: %w", name, err)
	}

	build := Build{
		ID:      buildInfo.ID,
		Name:    buildInfo.Name,
		BuildID: buildInfo.BuildID,
		Started: buildInfo.Started,
	}
	
	if buildInfo.Finished != nil {
		build.Finished = buildInfo.Finished
		duration := buildInfo.Finished.Sub(buildInfo.Started).Seconds()
		durationInt := int64(duration)
		build.Duration = &durationInt
	}

	return []Build{build}, nil
}