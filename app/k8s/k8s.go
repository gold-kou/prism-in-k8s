package k8s

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/gold-kou/prism-in-k8s/app/util"
	"github.com/pingcap/errors"
	"golang.org/x/xerrors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // to provide configuration
	restclient "k8s.io/client-go/rest"
)

const (
	localPrismImage = "my-local-image:latest"
	servicePort     = 80
)

var (
	errFailedToCreateClientSet  = errors.New("failed to create clientset")
	errFailedToCreateNameSpace  = errors.New("failed to create namespace")
	errFailedToCreateDeployment = errors.New("failed to create deployment")
	errFailedToCreateService    = errors.New("failed to create service")
	errFailedToDeleteNameSpace  = errors.New("failed to delete namespace")
	errFailedToDeleteDeployment = errors.New("failed to delete deployment")
	errFailedToDeleteService    = errors.New("failed to delete service")
	errFailedToListPods         = errors.New("failed to list pods")
	errFailedToGetLatestVersion = errors.New("failed to get latest version")
)

func CreateK8sResources(ctx context.Context, awsAccountID string, awsConfig aws.Config, kubeconfig *restclient.Config, namespaceName, resourceName string, istTest bool) error {
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToCreateClientSet, err)
	}

	err = createNamespace(ctx, k8sClientSet, namespaceName)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToCreateNameSpace, err)
	}

	err = crateDeployment(ctx, awsAccountID, awsConfig, k8sClientSet, namespaceName, resourceName, istTest)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToCreateDeployment, err)
	}

	err = createService(ctx, k8sClientSet, namespaceName, resourceName)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToCreateService, err)
	}

	return nil
}

func createNamespace(ctx context.Context, k8sClientSet *kubernetes.Clientset, namespaceName string) error {
	// get the latest istio version from istiod pod considering during upgrade, if not found return empty podList
	podList, err := k8sClientSet.CoreV1().Pods("istio-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app=istiod",
	})
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToListPods, err)
	}
	hyphenedVersions := []string{}
	for _, item := range podList.Items {
		hyphenedVersions = append(hyphenedVersions, item.ObjectMeta.Labels["istio.io/rev"])
	}
	latestVersion := getLatestVersion(hyphenedVersions)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToGetLatestVersion, err)
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
	_, err = k8sClientSet.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return xerrors.Errorf("%w: %w", errFailedToCreateNameSpace, err)
		}
		log.Println("[WARN] The namespace already exists")
	} else {
		log.Println("[INFO] Namespace is created successfully")
	}
	return nil
}

func crateDeployment(ctx context.Context, awsAccountID string, awsConfig aws.Config, k8sClientSet *kubernetes.Clientset, namespaceName, resourceName string, isTest bool) error {
	// Prism image
	prismImage := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s", awsAccountID, awsConfig.Region, resourceName)
	if isTest {
		prismImage = localPrismImage
	}

	// Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: util.Int32Ptr(1),
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
						"sidecar.istio.io/proxyCPULimit":                   params.IstioProxyCPU,
						"sidecar.istio.io/proxyMemoryLimit":                params.IstioProxyMemory,
						"traffic.sidecar.istio.io/includeOutboundIPRanges": "*",
						"proxy.istio.io/config":                            `{ "terminationDrainDuration": "30s" }`,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  resourceName,
							Image: prismImage,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: int32(params.PrismPort),
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(params.PrismCPU),
									corev1.ResourceMemory: resource.MustParse(params.PrismMemory),
								},
							},
						},
					},
					PriorityClassName: params.PriorityClassName,
				},
			},
		},
	}
	_, err := k8sClientSet.AppsV1().Deployments(namespaceName).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return xerrors.Errorf("%w: %w", errFailedToCreateDeployment, err)
		}
		log.Println("[WARN] The deployment already exists")
	} else {
		log.Println("[INFO] Deployment is created successfully")
	}
	return nil
}

func createService(ctx context.Context, k8sClientSet *kubernetes.Clientset, namespaceName, resourceName string) error {
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
					Port:       servicePort,
					TargetPort: intstr.FromInt(servicePort),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	_, err := k8sClientSet.CoreV1().Services(namespaceName).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return xerrors.Errorf("%w: %w", errFailedToCreateService, err)
		}
		log.Println("[WARN] The service already exists")
	} else {
		log.Println("[INFO] Service is created successfully")
	}
	return nil
}

func DeleteK8sResources(ctx context.Context, kubeconfig *restclient.Config, namespaceName, resourceName string) error {
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToCreateClientSet, err)
	}
	log.Println("[INFO] Clientset of k8s set up successfully")

	err = deleteService(ctx, k8sClientSet, namespaceName, resourceName)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToDeleteService, err)
	}

	err = deleteDeployment(ctx, k8sClientSet, namespaceName, resourceName)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToDeleteDeployment, err)
	}

	err = deleteNamespace(ctx, k8sClientSet, namespaceName)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToDeleteNameSpace, err)
	}

	return nil
}

func deleteService(ctx context.Context, k8sClientSet *kubernetes.Clientset, namespaceName, resourceName string) error {
	err := k8sClientSet.CoreV1().Services(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return xerrors.Errorf("%w: %w", errFailedToDeleteService, err)
		}
		log.Println("[WARN] The service is not found")
	} else {
		log.Println("[INFO] Service is deleted successfully")
	}
	return nil
}

func deleteDeployment(ctx context.Context, k8sClientSet *kubernetes.Clientset, namespaceName, resourceName string) error {
	err := k8sClientSet.AppsV1().Deployments(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return xerrors.Errorf("%w: %w", errFailedToDeleteDeployment, err)
		}
		log.Println("[WARN] The Deployment is not found")
	} else {
		log.Println("[INFO] Deployment is deleted successfully")
	}
	return nil
}

func deleteNamespace(ctx context.Context, k8sClientSet *kubernetes.Clientset, namespaceName string) error {
	err := k8sClientSet.CoreV1().Namespaces().Delete(ctx, namespaceName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return xerrors.Errorf("%w: %w", errFailedToDeleteNameSpace, err)
		}
		log.Println("[WARN] The Namespace is not found")
	} else {
		log.Println("[INFO] Namespace is deleted successfully")
	}
	return nil
}

func parseVersion(version string) ([]int, error) {
	versions := 3

	// convert "x-y-z" to [x, y, z]
	parts := strings.Split(version, "-")
	if len(parts) != versions {
		return nil, xerrors.Errorf("invalid version format: %s", version)
	}

	intParts := make([]int, len(parts))
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, xerrors.Errorf("invalid number in version: %s", part)
		}
		intParts[i] = num
	}
	return intParts, nil
}

func compareVersions(v1, v2 []int) int {
	// return 1 if v1 > v2, -1 if v1 < v2, 0 if v1 == v2
	for i := range v1 {
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

func getLatestVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}

	// init max with the zero index element
	maxVersion := versions[0]
	// ignore err
	maxVersionParts, _ := parseVersion(maxVersion)

	// compare all versions
	for _, version := range versions[1:] {
		// ignore err
		versionParts, _ := parseVersion(version)

		if compareVersions(versionParts, maxVersionParts) > 0 {
			maxVersion = version
			maxVersionParts = versionParts
		}
	}

	return maxVersion
}
