package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ANSI color codes
const (
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorBlue   = "\033[0;34m"
	colorYellow = "\033[1;33m"
	colorCyan   = "\033[0;36m"
	colorReset  = "\033[0m"
)

// ResourceMapper holds the Kubernetes client and context
type ResourceMapper struct {
	clientset *kubernetes.Clientset
	ctx       context.Context
}

// stringSliceFlag implements flag.Value interface for string slice flags
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// NewResourceMapper creates a new ResourceMapper instance
func NewResourceMapper() (*ResourceMapper, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("error getting home directory: %v", err)
		}
		kubeconfig = homeDir + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %v", err)
	}

	return &ResourceMapper{
		clientset: clientset,
		ctx:       context.Background(),
	}, nil
}

// printLine prints a horizontal line
func (rm *ResourceMapper) printLine() {
	fmt.Println(strings.Repeat("-", 80))
}

// createArrow creates an ASCII arrow of specified length
func (rm *ResourceMapper) createArrow(length int) string {
	return strings.Repeat("-", length) + ">"
}

// getResources gets all resources in a namespace
func (rm *ResourceMapper) getResources(namespace string) error {
	fmt.Printf("%sResources in namespace: %s%s\n", colorGreen, namespace, colorReset)

	// Get deployments
	fmt.Printf("\n%sDeployments:%s\n", colorYellow, colorReset)
	deployments, err := rm.clientset.AppsV1().Deployments(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting deployments: %v", err)
	}
	for _, deploy := range deployments.Items {
		fmt.Printf("%s %d %d\n", deploy.Name, *deploy.Spec.Replicas, deploy.Status.AvailableReplicas)
	}

	// Get HPA
	fmt.Printf("\n%sHpa:%s\n", colorYellow, colorReset)
	hpas, err := rm.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting HPAs: %v", err)
	}
	for _, hpa := range hpas.Items {
		fmt.Printf("%s ", hpa.Name)
		for _, metric := range hpa.Spec.Metrics {
			if metric.Resource != nil {
				fmt.Printf("%s %d ", metric.Resource.Name, *metric.Resource.Target.AverageUtilization)
			}
		}
		fmt.Println()
	}

	// Get services
	fmt.Printf("\n%sServices:%s\n", colorYellow, colorReset)
	services, err := rm.clientset.CoreV1().Services(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting services: %v", err)
	}
	for _, svc := range services.Items {
		fmt.Printf("%s %s %s %v\n", svc.Name, svc.Spec.Type, svc.Spec.ClusterIP, svc.Spec.ExternalIPs)
	}

	// Get Ingresses
	fmt.Printf("\n%sIngress:%s\n", colorYellow, colorReset)
	ingresses, err := rm.clientset.NetworkingV1().Ingresses(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting ingresses: %v", err)
	}
	for _, ing := range ingresses.Items {
		hosts := []string{}
		for _, rule := range ing.Spec.Rules {
			hosts = append(hosts, rule.Host)
		}
		fmt.Printf("%s %s\n", ing.Name, strings.Join(hosts, ","))
	}

	// Get pods
	fmt.Printf("\n%sPods:%s\n", colorYellow, colorReset)
	pods, err := rm.clientset.CoreV1().Pods(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting pods: %v", err)
	}
	for _, pod := range pods.Items {
		fmt.Printf("%s %s %s\n", pod.Name, pod.Status.Phase, pod.Spec.NodeName)
	}

	// Get configmaps
	fmt.Printf("\n%sConfigMaps:%s\n", colorYellow, colorReset)
	configmaps, err := rm.clientset.CoreV1().ConfigMaps(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting configmaps: %v", err)
	}
	for _, cm := range configmaps.Items {
		fmt.Printf("%s\n", cm.Name)
	}

	return nil
}

// mapServiceConnections maps service connections in a namespace
func (rm *ResourceMapper) mapServiceConnections(namespace string) error {
	fmt.Printf("\n%sService connections in namespace: %s%s\n", colorBlue, namespace, colorReset)

	services, err := rm.clientset.CoreV1().Services(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting services: %v", err)
	}

	for _, service := range services.Items {
		fmt.Printf("\n%sService: %s%s\n", colorYellow, service.Name, colorReset)

		if len(service.Spec.Selector) > 0 {
			fmt.Printf("├── Selectors: %v\n", service.Spec.Selector)

			labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{
				MatchLabels: service.Spec.Selector,
			})
			pods, err := rm.clientset.CoreV1().Pods(namespace).List(rm.ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				return fmt.Errorf("error getting pods for service %s: %v", service.Name, err)
			}

			if len(pods.Items) > 0 {
				fmt.Println("└── Connected Pods:")
				for _, pod := range pods.Items {
					fmt.Printf("    %s %s\n", rm.createArrow(4), pod.Name)
				}
			}
		}
	}

	return nil
}

// showResourceRelationships shows resource relationships in a namespace
func (rm *ResourceMapper) showResourceRelationships(namespace string) error {
	fmt.Printf("\n%sResource relationships in namespace: %s%s\n\n", colorBlue, namespace, colorReset)

	fmt.Println("External Traffic")
	fmt.Println("│")

	// Handle Ingresses
	ingresses, err := rm.clientset.NetworkingV1().Ingresses(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting ingresses: %v", err)
	}

	if len(ingresses.Items) > 0 {
		fmt.Println("▼")
		fmt.Println("[Ingress Layer]")
		for _, ingress := range ingresses.Items {
			fmt.Printf("├── %s\n", ingress.Name)
			for _, rule := range ingress.Spec.Rules {
				if rule.HTTP != nil {
					for _, path := range rule.HTTP.Paths {
						fmt.Printf("│   %s Service: %s\n", rm.createArrow(4), path.Backend.Service.Name)
					}
				}
			}
		}
		fmt.Println("│")
	}

	// Handle Services
	fmt.Println("▼")
	fmt.Println("[Service Layer]")
	services, err := rm.clientset.CoreV1().Services(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting services: %v", err)
	}

	for _, service := range services.Items {
		fmt.Printf("├── %s\n", service.Name)

		if len(service.Spec.Selector) > 0 {
			labelSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{
				MatchLabels: service.Spec.Selector,
			})
			pods, err := rm.clientset.CoreV1().Pods(namespace).List(rm.ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				return fmt.Errorf("error getting pods for service %s: %v", service.Name, err)
			}

			for _, pod := range pods.Items {
				fmt.Printf("│   %s Pod: %s\n", rm.createArrow(4), pod.Name)
			}
		}
	}

	return nil
}

// showConfigMapUsage shows ConfigMap usage in a namespace
func (rm *ResourceMapper) showConfigMapUsage(namespace string) error {
	fmt.Printf("\n%sConfigMap usage in namespace: %s%s\n", colorCyan, namespace, colorReset)

	configMaps, err := rm.clientset.CoreV1().ConfigMaps(namespace).List(rm.ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting configmaps: %v", err)
	}

	for _, cm := range configMaps.Items {
		fmt.Printf("\nConfigMap: %s\n", cm.Name)

		pods, err := rm.clientset.CoreV1().Pods(namespace).List(rm.ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("error getting pods: %v", err)
		}

		usagePods := make(map[string][]string)
		for _, pod := range pods.Items {
			// Check volume mounts
			for _, volume := range pod.Spec.Volumes {
				if volume.ConfigMap != nil && volume.ConfigMap.Name == cm.Name {
					usagePods[pod.Name] = append(usagePods[pod.Name], "Mounted as volume")
				}
			}

			// Check containers for envFrom and env
			for _, container := range pod.Spec.Containers {
				for _, envFrom := range container.EnvFrom {
					if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == cm.Name {
						usagePods[pod.Name] = append(usagePods[pod.Name], "Used in envFrom")
					}
				}

				for _, env := range container.Env {
					if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil &&
						env.ValueFrom.ConfigMapKeyRef.Name == cm.Name {
						usagePods[pod.Name] = append(usagePods[pod.Name], "Used in environment variables")
					}
				}
			}
		}

		if len(usagePods) > 0 {
			fmt.Println("└── Used by pods:")
			podNames := make([]string, 0, len(usagePods))
			for podName := range usagePods {
				podNames = append(podNames, podName)
			}
			sort.Strings(podNames)

			for _, podName := range podNames {
				fmt.Printf("    %s %s\n", rm.createArrow(4), podName)
				for _, usage := range usagePods[podName] {
					fmt.Printf("        - %s\n", usage)
				}
			}
		}
	}

	return nil
}

// processNamespace processes a single namespace
func (rm *ResourceMapper) processNamespace(namespace string) error {
	rm.printLine()
	fmt.Printf("%sAnalyzing namespace: %s%s\n", colorRed, namespace, colorReset)
	rm.printLine()

	if err := rm.getResources(namespace); err != nil {
		return err
	}

	if err := rm.mapServiceConnections(namespace); err != nil {
		return err
	}

	if err := rm.showResourceRelationships(namespace); err != nil {
		return err
	}

	if err := rm.showConfigMapUsage(namespace); err != nil {
		return err
	}

	rm.printLine()
	return nil
}

func main() {
	var (
		namespace = flag.String("n", "", "Process only the specified namespace")
		excludeNs stringSliceFlag
		help      = flag.Bool("h", false, "Show help message")
	)

	flag.StringVar(namespace, "namespace", "", "Process only the specified namespace")
	flag.Var(&excludeNs, "exclude-ns", "Exclude specified namespaces")
	flag.BoolVar(help, "help", false, "Show help message")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	rm, err := NewResourceMapper()
	if err != nil {
		fmt.Printf("%sError initializing resource mapper: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	fmt.Printf("%sKubernetes Resource Mapper%s\n", colorGreen, colorReset)
	rm.printLine()

	var namespaces []string
	if *namespace != "" {
		// Check if specified namespace exists
		_, err := rm.clientset.CoreV1().Namespaces().Get(rm.ctx, *namespace, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("%sError: Namespace '%s' not found%s\n", colorRed, *namespace, colorReset)
			os.Exit(1)
		}
		namespaces = []string{*namespace}
	} else {
		// Get all namespaces
		nsList, err := rm.clientset.CoreV1().Namespaces().List(rm.ctx, metav1.ListOptions{})
		if err != nil {
			fmt.Printf("%sError getting namespaces: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}

		// Filter out excluded namespaces
		for _, ns := range nsList.Items {
			excluded := false
			for _, excludedNs := range excludeNs {
				if ns.Name == excludedNs {
					excluded = true
					break
				}
			}
			if !excluded {
				namespaces = append(namespaces, ns.Name)
			}
		}
	}

	// Process namespaces
	for _, ns := range namespaces {
		if err := rm.processNamespace(ns); err != nil {
			fmt.Printf("%sError processing namespace %s: %v%s\n", colorRed, ns, err, colorReset)
			continue
		}
	}

	fmt.Printf("%sResource mapping complete!%s\n", colorGreen, colorReset)
}
