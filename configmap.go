// Package configmap implements storage using Kubernetes ConfigMaps.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	configMapTimeout5s  = 5 * time.Second
	configMapTimeout10s = 10 * time.Second
	configMapMaxRetries = 5
)

// BuildInfo represents the structure stored in ConfigMap
type BuildInfo struct {
	Name     string     `json:"name"`
	BuildID  string     `json:"build_id"`
	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`
	ID       int        `json:"id"`
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

// StartBuild records the start of a build, retrying on resource version conflicts
func (cms *ConfigMapStorage) StartBuild(name, buildID string) (int, error) {
	for attempt := 0; attempt < configMapMaxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), configMapTimeout10s)

		cm, err := cms.client.CoreV1().ConfigMaps(cms.namespace).Get(ctx, cms.configMap, metav1.GetOptions{})
		if err != nil {
			if !k8serrors.IsNotFound(err) {
				cancel()
				return 0, fmt.Errorf("failed to get ConfigMap: %w", err)
			}
			cm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cms.configMap,
					Namespace: cms.namespace,
				},
				Data: make(map[string]string),
			}
			cm, err = cms.client.CoreV1().ConfigMaps(cms.namespace).Create(ctx, cm, metav1.CreateOptions{})
			if err != nil {
				cancel()
				if k8serrors.IsAlreadyExists(err) {
					continue
				}
				return 0, fmt.Errorf("failed to create ConfigMap: %w", err)
			}
		}

		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}

		buildInfo := BuildInfo{
			Name:    name,
			BuildID: buildID,
			Started: time.Now(),
			ID:      int(time.Now().Unix()),
		}

		data, err := json.Marshal(buildInfo)
		if err != nil {
			cancel()
			return 0, fmt.Errorf("failed to marshal build info: %w", err)
		}

		cm.Data[name] = string(data)

		_, err = cms.client.CoreV1().ConfigMaps(cms.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		cancel()
		if err != nil {
			if k8serrors.IsConflict(err) {
				continue
			}
			return 0, fmt.Errorf("failed to update ConfigMap: %w", err)
		}

		return buildInfo.ID, nil
	}
	return 0, fmt.Errorf("failed to update ConfigMap after %d retries", configMapMaxRetries)
}

// FinishBuild records the completion of a build, retrying on resource version conflicts
func (cms *ConfigMapStorage) FinishBuild(name, buildID string) error {
	for attempt := 0; attempt < configMapMaxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), configMapTimeout10s)

		cm, err := cms.client.CoreV1().ConfigMaps(cms.namespace).Get(ctx, cms.configMap, metav1.GetOptions{})
		if err != nil {
			cancel()
			return fmt.Errorf("failed to get ConfigMap: %w", err)
		}

		if cm.Data == nil {
			cancel()
			return fmt.Errorf("no build data found for name: %s", name)
		}

		data, exists := cm.Data[name]
		if !exists {
			cancel()
			return fmt.Errorf("no build found for name: %s", name)
		}

		var buildInfo BuildInfo
		if err := json.Unmarshal([]byte(data), &buildInfo); err != nil {
			cancel()
			return fmt.Errorf("failed to unmarshal build info: %w", err)
		}

		if buildInfo.BuildID != buildID {
			cancel()
			return fmt.Errorf("build_id mismatch: expected %s, got %s", buildInfo.BuildID, buildID)
		}

		now := time.Now()
		buildInfo.Finished = &now

		updatedData, err := json.Marshal(buildInfo)
		if err != nil {
			cancel()
			return fmt.Errorf("failed to marshal updated build info: %w", err)
		}

		cm.Data[name] = string(updatedData)

		_, err = cms.client.CoreV1().ConfigMaps(cms.namespace).Update(ctx, cm, metav1.UpdateOptions{})
		cancel()
		if err != nil {
			if k8serrors.IsConflict(err) {
				continue
			}
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}

		return nil
	}
	return fmt.Errorf("failed to update ConfigMap after %d retries", configMapMaxRetries)
}

// HealthCheck verifies the ConfigMap storage is accessible
func (cms *ConfigMapStorage) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), configMapTimeout5s)
	defer cancel()

	_, err := cms.client.CoreV1().ConfigMaps(cms.namespace).Get(ctx, cms.configMap, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("failed to access ConfigMap: %w", err)
		}
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cms.configMap,
				Namespace: cms.namespace,
			},
			Data: make(map[string]string),
		}
		_, err = cms.client.CoreV1().ConfigMaps(cms.namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}
	}
	return nil
}

// GetBuildInfo retrieves current build information for a name
func (cms *ConfigMapStorage) GetBuildInfo(name string) (*BuildInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), configMapTimeout5s)
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
	ctx, cancel := context.WithTimeout(context.Background(), configMapTimeout5s)
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
			duration := int64(buildInfo.Finished.Sub(buildInfo.Started).Seconds())
			build.Duration = &duration
		}

		projects = append(projects, ProjectSummary{
			Name:        name,
			LatestBuild: build,
			BuildCount:  1, // ConfigMap only stores latest build per project
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
		duration := int64(buildInfo.Finished.Sub(buildInfo.Started).Seconds())
		build.Duration = &duration
	}

	return []Build{build}, nil
}
