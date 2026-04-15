package k8s

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gold-kou/prism-in-k8s/app/params"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // to provide configuration
	restclient "k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

const (
	localPrismImage = "my-local-image:v1"
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
	errNoValidVersionFound      = errors.New("no valid version found")
	errInvalidVersionFormat     = errors.New("invalid version format")
	errInvalidNumberInVersion   = errors.New("invalid number in version")
)

func CreateK8sResources(ctx context.Context, awsAccountID string, awsConfig aws.Config, kubeconfig *restclient.Config, namespaceName, resourceName string, istioMode, isTest bool) error {
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToCreateClientSet, err)
	}

	err = createNamespace(ctx, k8sClientSet, namespaceName, istioMode)
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToCreateNameSpace, err)
	}

	err = createDeployment(ctx, awsAccountID, awsConfig, k8sClientSet, namespaceName, resourceName, istioMode, isTest)
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToCreateDeployment, err)
	}

	err = createService(ctx, k8sClientSet, namespaceName, resourceName)
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToCreateService, err)
	}

	return nil
}

func createNamespace(ctx context.Context, k8sClientSet *kubernetes.Clientset, namespaceName string, istioMode bool) error {
	// Namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespaceName,
			Labels: map[string]string{},
		},
	}

	// get the latest istio version from istiod pod considering during upgrade, if not found return empty podList
	if istioMode {
		podList, err := k8sClientSet.CoreV1().Pods("istio-system").List(ctx, metav1.ListOptions{
			LabelSelector: "app=istiod",
		})
		if err != nil {
			return fmt.Errorf("%w: %w", errFailedToListPods, err)
		}
		hyphenedVersions := []string{}
		for _, item := range podList.Items {
			hyphenedVersions = append(hyphenedVersions, item.ObjectMeta.Labels["istio.io/rev"])
		}
		latestVersion, err := getLatestVersion(hyphenedVersions)
		if err != nil {
			return fmt.Errorf("%w: %w", errFailedToGetLatestVersion, err)
		}
		namespace.ObjectMeta.Labels["istio.io/rev"] = latestVersion
	}

	_, err := k8sClientSet.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("%w: %w", errFailedToCreateNameSpace, err)
		}
		log.Println("[WARN] The namespace already exists")
	} else {
		log.Println("[INFO] Namespace is created successfully")
	}
	return nil
}

func createDeployment(ctx context.Context, awsAccountID string, awsConfig aws.Config, k8sClientSet *kubernetes.Clientset, namespaceName, resourceName string, istioMode, isTest bool) error {
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
			Replicas: ptr.To[int32](1),
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
					Annotations: map[string]string{},
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

	deployment.Spec.Template.Spec.Affinity = buildAffinity(resourceName)

	if isTest {
		// to get image from local
		deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever
	}

	if istioMode {
		deployment.Spec.Template.ObjectMeta.Annotations["sidecar.istio.io/inject"] = "true"
		deployment.Spec.Template.ObjectMeta.Annotations["sidecar.istio.io/proxyCPULimit"] = params.IstioProxyCPU
		deployment.Spec.Template.ObjectMeta.Annotations["sidecar.istio.io/proxyMemoryLimit"] = params.IstioProxyMemory
		deployment.Spec.Template.ObjectMeta.Annotations["traffic.sidecar.istio.io/includeOutboundIPRanges"] = "*"
		deployment.Spec.Template.ObjectMeta.Annotations["proxy.istio.io/config"] = `{ "terminationDrainDuration": "30s" }`
	}

	_, err := k8sClientSet.AppsV1().Deployments(namespaceName).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("%w: %w", errFailedToCreateDeployment, err)
		}
		log.Println("[WARN] The deployment already exists")
	} else {
		log.Println("[INFO] Deployment is created successfully")
	}
	return nil
}

func buildAffinity(resourceName string) *corev1.Affinity {
	hasNodeAffinity := len(params.NodeAffinityMatchExpressions) > 0
	hasPodAntiAffinity := params.PodAntiAffinityTopologyKey != ""

	if !hasNodeAffinity && !hasPodAntiAffinity {
		return nil
	}

	affinity := &corev1.Affinity{}

	if hasNodeAffinity {
		matchExpressions := make([]corev1.NodeSelectorRequirement, 0, len(params.NodeAffinityMatchExpressions))
		for _, expr := range params.NodeAffinityMatchExpressions {
			matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
				Key:      expr.Key,
				Operator: corev1.NodeSelectorOperator(expr.Operator),
				Values:   expr.Values,
			})
		}
		affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: matchExpressions,
					},
				},
			},
		}
	}

	if hasPodAntiAffinity {
		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "app",
								Operator: metav1.LabelSelectorOpIn,
								Values:   []string{resourceName},
							},
						},
					},
					TopologyKey: params.PodAntiAffinityTopologyKey,
				},
			},
		}
	}

	return affinity
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
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("%w: %w", errFailedToCreateService, err)
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
		return fmt.Errorf("%w: %w", errFailedToCreateClientSet, err)
	}
	log.Println("[INFO] Clientset of k8s set up successfully")

	err = deleteService(ctx, k8sClientSet, namespaceName, resourceName)
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToDeleteService, err)
	}

	err = deleteDeployment(ctx, k8sClientSet, namespaceName, resourceName)
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToDeleteDeployment, err)
	}

	err = deleteNamespace(ctx, k8sClientSet, namespaceName)
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToDeleteNameSpace, err)
	}

	return nil
}

func deleteService(ctx context.Context, k8sClientSet *kubernetes.Clientset, namespaceName, resourceName string) error {
	err := k8sClientSet.CoreV1().Services(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("%w: %w", errFailedToDeleteService, err)
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
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("%w: %w", errFailedToDeleteDeployment, err)
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
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("%w: %w", errFailedToDeleteNameSpace, err)
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
		return nil, fmt.Errorf("%w: %s", errInvalidVersionFormat, version)
	}

	intParts := make([]int, len(parts))
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errInvalidNumberInVersion, part)
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

// return the latest version of label value of istio.io/rev
func getLatestVersion(versions []string) (string, error) {
	if len(versions) == 0 {
		return "", nil
	}

	maxVersion := ""
	var maxVersionParts []int
	var nonVersionRevision string

	for _, version := range versions {
		versionParts, err := parseVersion(version)
		if err != nil {
			// Non-version revision names like "default" are valid Istio revisions
			log.Printf("[WARN] Non-version revision %q found, treating as valid revision", version)
			if nonVersionRevision == "" {
				nonVersionRevision = version
			}
			continue
		}

		if maxVersionParts == nil || compareVersions(versionParts, maxVersionParts) > 0 {
			maxVersion = version
			maxVersionParts = versionParts
		}
	}

	// Prefer versioned revisions over non-version ones
	if maxVersion != "" {
		return maxVersion, nil
	}
	if nonVersionRevision != "" {
		return nonVersionRevision, nil
	}

	return "", fmt.Errorf("%w: %v", errNoValidVersionFound, versions)
}
