package client

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient wraps the Kubernetes clientset and configuration
type K8sClient struct {
	Clientset *kubernetes.Clientset
	Config    *rest.Config
}

// NewK8sClient creates a new Kubernetes client using provided or default configuration
func NewK8sClient(kubeconfigPath string) (*K8sClient, error) {
	config, err := getClientConfig(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	return &K8sClient{
		Clientset: clientset,
		Config:    config,
	}, nil
}

// getClientConfig returns a Kubernetes client configuration
func getClientConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get user home directory: %v", err)
			}
			kubeconfigPath = filepath.Join(homeDir, ".kube", "config")
		}
	}

	// Try loading from kubeconfig file first
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		// If loading from kubeconfig fails, try in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster or kubeconfig: %v", err)
		}
	}

	return config, nil
}

// Cleanup performs any necessary cleanup operations
func (c *K8sClient) Cleanup() {
	// Add any cleanup operations if needed
}

// IsConnected checks if the client can connect to the cluster
func (c *K8sClient) IsConnected() error {
	_, err := c.Clientset.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %v", err)
	}
	return nil
}
