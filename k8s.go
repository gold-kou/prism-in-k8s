package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/pingcap/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func createK8sResources(ctx context.Context) error {
	// create clientset using kubeconfig
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %v", err)
	}

	// get the latest istio version from istiod pod considering during upgrade
	podList, err := clientset.CoreV1().Pods("istio-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app=istiod",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}
	hyphenedVersions := []string{}
	for _, item := range podList.Items {
		hyphenedVersions = append(hyphenedVersions, item.ObjectMeta.Labels["istio.io/rev"])
	}
	latestVersion, err := getLatestVersion(hyphenedVersions)
	if err != nil {
		return fmt.Errorf("failed to get the latest version: %v", err)
	}

	// Namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
			Labels: map[string]string{
				"istio.io/rev": latestVersion,
			},
		},
	}
	_, err = clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create namespace: %v", err)
		}
		log.Println("[WARN] The namespace already exists")
	} else {
		log.Println("[INFO] Namespace is created successfully")
	}

	// Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": resourceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": resourceName,
					},
					Annotations: map[string]string{
						"sidecar.istio.io/inject":                          "true",
						"sidecar.istio.io/proxyCPULimit":                   istioProxyCPU,
						"sidecar.istio.io/proxyMemoryLimit":                istioProxyMemory,
						"traffic.sidecar.istio.io/includeOutboundIPRanges": "*",
						"proxy.istio.io/config":                            `{ "terminationDrainDuration": "30s" }`,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  resourceName,
							Image: fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s", awsAccountID, awsConfig.Region, resourceName),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(prismPort),
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(prismCPU),
									corev1.ResourceMemory: resource.MustParse(prismMemory),
								},
							},
						},
					},
				},
			},
		},
	}
	_, err = clientset.AppsV1().Deployments(namespaceName).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create deployment: %v", err)
		}
		log.Println("[WARN] The deployment already exists")
	} else {
		log.Println("[INFO] Deployment is created successfully")
	}

	// Service
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": resourceName,
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	_, err = clientset.CoreV1().Services(namespaceName).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create service: %v", err)
		}
		log.Println("[WARN] The service already exists")
	} else {
		log.Println("[INFO] Service is created successfully")
	}
	return nil
}

func deleteK8sResources(ctx context.Context) error {
	// create clientset using kubeconfig
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("clientset creation error: %v", err)
	}
	log.Println("[INFO] Clientset of k8s set up successfully")

	// Service
	err = clientset.CoreV1().Services(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Service: %v", err)
		}
		log.Println("[WARN] The service is not found")
	} else {
		log.Println("[INFO] Service is deleted successfully")
	}

	// Deployment
	err = clientset.AppsV1().Deployments(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Deployment: %v", err)
		}
		log.Println("[WARN] The Deployment is not found")
	} else {
		log.Println("[INFO] Deployment is deleted successfully")
	}

	// Namespace
	err = clientset.CoreV1().Namespaces().Delete(ctx, namespaceName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Namespace: %v", err)
		}
		log.Println("[WARN] The Namespace is not found")
	} else {
		log.Println("[INFO] Namespace is deleted successfully")
	}
	return nil
}

func parseVersion(version string) ([]int, error) {
	// convert "x-y-z" to [x, y, z]
	parts := strings.Split(version, "-")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}

	intParts := make([]int, len(parts))
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid number in version: %s", part)
		}
		intParts[i] = num
	}
	return intParts, nil
}

func compareVersions(v1, v2 []int) int {
	// return 1 if v1 > v2, -1 if v1 < v2, 0 if v1 == v2
	for i := 0; i < len(v1); i++ {
		// if just one part is greater, the version is greater
		if v1[i] > v2[i] {
			return 1
		} else if v1[i] < v2[i] {
			return -1
		}
	}
	// if all parts are equal, the versions are equal
	return 0
}

func getLatestVersion(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", nil
	}

	// init max with the zero index element
	maxVersion := versions[0]
	maxVersionParts, err := parseVersion(maxVersion)
	if err != nil {
		return "", err
	}

	// compare all versions
	for _, version := range versions[1:] {
		versionParts, err := parseVersion(version)
		if err != nil {
			return "", err
		}

		if compareVersions(versionParts, maxVersionParts) > 0 {
			maxVersion = version
			maxVersionParts = versionParts
		}
	}

	return maxVersion, nil
}
