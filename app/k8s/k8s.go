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
	// Affinity
	fmt.Println(*params.DeploymentAffinity)
	if params.DeploymentAffinity != nil {
		deployment.Spec.Template.Spec.Affinity = makeAffinityParams(params.DeploymentAffinity)
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

func makeAffinityParams(affinity *params.Affinity) *corev1.Affinity {
	if affinity == nil {
		return nil
	}

	return &corev1.Affinity{
		NodeAffinity:    makeNodeAffinity(affinity.NodeAffinity),
		PodAffinity:     makePodAffinity(affinity.PodAffinity),
		PodAntiAffinity: makePodAntiAffinity(affinity.PodAntiAffinity),
	}
}

func makeNodeAffinity(nodeAffinity *params.NodeAffinity) *corev1.NodeAffinity {
	if nodeAffinity == nil {
		return nil
	}

	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: makeNodeSelectorTerms(nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution),
		},
		PreferredDuringSchedulingIgnoredDuringExecution: makePreferredSchedulingTerms(nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution),
	}
}

func makePodAffinity(podAffinity *params.PodAffinity) *corev1.PodAffinity {
	if podAffinity == nil {
		return nil
	}

	return &corev1.PodAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution:  makePodAffinityTerms(podAffinity.RequiredDuringSchedulingIgnoredDuringExecution),
		PreferredDuringSchedulingIgnoredDuringExecution: makeWeightedPodAffinityTerms(podAffinity.PreferredDuringSchedulingIgnoredDuringExecution),
	}
}

func makePodAntiAffinity(podAntiAffinity *params.PodAntiAffinity) *corev1.PodAntiAffinity {
	if podAntiAffinity == nil {
		return nil
	}

	return &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution:  makePodAffinityTerms(podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution),
		PreferredDuringSchedulingIgnoredDuringExecution: makeWeightedPodAffinityTerms(podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution),
	}
}

func makeNodeSelectorTerms(terms []params.NodeSelectorTerm) []corev1.NodeSelectorTerm {
	var result []corev1.NodeSelectorTerm
	for _, term := range terms {
		result = append(result, corev1.NodeSelectorTerm{
			MatchExpressions: makeNodeSelectorRequirements(term.MatchExpressions),
		})
	}
	return result
}

func makePreferredSchedulingTerms(terms []params.PreferredSchedulingTerm) []corev1.PreferredSchedulingTerm {
	var result []corev1.PreferredSchedulingTerm
	for _, term := range terms {
		result = append(result, corev1.PreferredSchedulingTerm{
			Weight: term.Weight,
			Preference: corev1.NodeSelectorTerm{
				MatchExpressions: makeNodeSelectorRequirements(term.Preference.MatchExpressions),
			},
		})
	}
	return result
}

func makePodAffinityTerms(terms []params.PodAffinityTerm) []corev1.PodAffinityTerm {
	var result []corev1.PodAffinityTerm
	for _, term := range terms {
		result = append(result, corev1.PodAffinityTerm{
			LabelSelector: makeLabelSelector(term.LabelSelector),
			TopologyKey:   term.TopologyKey,
		})
	}
	return result
}

func makeWeightedPodAffinityTerms(terms []params.WeightedPodAffinityTerm) []corev1.WeightedPodAffinityTerm {
	var result []corev1.WeightedPodAffinityTerm
	for _, term := range terms {
		result = append(result, corev1.WeightedPodAffinityTerm{
			Weight: term.Weight,
			PodAffinityTerm: corev1.PodAffinityTerm{
				LabelSelector: makeLabelSelector(term.PodAffinityTerm.LabelSelector),
				TopologyKey:   term.PodAffinityTerm.TopologyKey,
			},
		})
	}
	return result
}

func makeNodeSelectorRequirements(reqs []params.NodeSelectorRequirement) []corev1.NodeSelectorRequirement {
	var result []corev1.NodeSelectorRequirement
	for _, req := range reqs {
		result = append(result, corev1.NodeSelectorRequirement{
			Key:      req.Key,
			Operator: corev1.NodeSelectorOperator(req.Operator),
			Values:   req.Values,
		})
	}
	return result
}

func makeLabelSelector(selector *params.LabelSelector) *metav1.LabelSelector {
	if selector == nil {
		return nil
	}
	return &metav1.LabelSelector{
		MatchExpressions: makeLabelSelectorRequirements(selector.MatchExpressions),
	}
}

func makeLabelSelectorRequirements(reqs []params.LabelSelectorRequirement) []metav1.LabelSelectorRequirement {
	var result []metav1.LabelSelectorRequirement
	for _, req := range reqs {
		result = append(result, metav1.LabelSelectorRequirement{
			Key:      req.Key,
			Operator: metav1.LabelSelectorOperator(req.Operator),
			Values:   req.Values,
		})
	}
	return result
}

// func makeAffinityParams(p params.Affinity) *corev1.Affinity {
// 	affinity := &corev1.Affinity{}
// 	if p.NodeAffinity != nil {
// 		nodeAffinity := &corev1.NodeAffinity{}
// 		if p.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
// 			requiredDuringScgedulingIgnoredDuringExecution := &corev1.NodeSelector{
// 				NodeSelectorTerms: []corev1.NodeSelectorTerm{
// 					{
// 						MatchExpressions: []corev1.NodeSelectorRequirement{
// 							{
// 								Key:      p.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].MatchExpressions[0].Key,
// 								Operator: p.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].MatchExpressions[0].Operator,
// 								Values:   p.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].MatchExpressions[0].Values,
// 							},
// 						},
// 					},
// 				},
// 			}
// 			nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = requiredDuringScgedulingIgnoredDuringExecution
// 		}
// 		if p.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
// 			preferredDuringSchedulingIgnoredDuringExecution := []corev1.PreferredSchedulingTerm{
// 				{
// 					Weight: p.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight,
// 					Preference: corev1.NodeSelectorTerm{
// 						MatchExpressions: []corev1.NodeSelectorRequirement{
// 							{
// 								Key:      p.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Key,
// 								Operator: p.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Operator,
// 								Values:   p.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values,
// 							},
// 						},
// 					},
// 				},
// 			}
// 			nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = preferredDuringSchedulingIgnoredDuringExecution
// 		}
// 		affinity.NodeAffinity = nodeAffinity
// 	}
// 	if p.PodAffinity != nil {
// 		podAffinity := &corev1.PodAffinity{}
// 		if p.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
// 			requiredDuringSchedulingIgnoredDuringExecution := []corev1.PodAffinityTerm{
// 				{
// 					LabelSelector: &metav1.LabelSelector{
// 						MatchExpressions: []metav1.LabelSelectorRequirement{
// 							{
// 								Key:      p.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions[0].Key,
// 								Operator: p.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions[0].Operator,
// 								Values:   p.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions[0].Values,
// 							},
// 						},
// 					},
// 					TopologyKey: p.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey,
// 				},
// 			}
// 			podAffinity.RequiredDuringSchedulingIgnoredDuringExecution = requiredDuringSchedulingIgnoredDuringExecution
// 		}
// 		if p.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
// 			preferredDuringSchedulingIgnoredDuringExecution := []corev1.WeightedPodAffinityTerm{
// 				{
// 					Weight: p.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight,
// 					PodAffinityTerm: corev1.PodAffinityTerm{
// 						LabelSelector: &metav1.LabelSelector{
// 							MatchExpressions: []metav1.LabelSelectorRequirement{
// 								{
// 									Key:      p.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Key,
// 									Operator: p.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Operator,
// 									Values:   p.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Values,
// 								},
// 							},
// 						},
// 						TopologyKey: p.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey,
// 					},
// 				},
// 			}
// 			podAffinity.PreferredDuringSchedulingIgnoredDuringExecution = preferredDuringSchedulingIgnoredDuringExecution
// 		}
// 		affinity.PodAffinity = podAffinity
// 	}
// 	if p.PodAntiAffinity != nil {
// 		podAntiAffinity := &corev1.PodAntiAffinity{}
// 		if p.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
// 			requiredDuringSchedulingIgnoredDuringExecution := []corev1.PodAffinityTerm{
// 				{
// 					LabelSelector: &metav1.LabelSelector{
// 						MatchExpressions: []metav1.LabelSelectorRequirement{
// 							{
// 								Key:      p.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions[0].Key,
// 								Operator: p.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions[0].Operator,
// 								Values:   p.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions[0].Values,
// 							},
// 						},
// 					},
// 					TopologyKey: p.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey,
// 				},
// 			}
// 			podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = requiredDuringSchedulingIgnoredDuringExecution
// 		}
// 		if p.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution != nil {
// 			preferredDuringSchedulingIgnoredDuringExecution := []corev1.WeightedPodAffinityTerm{
// 				{
// 					Weight: p.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight,
// 					PodAffinityTerm: corev1.PodAffinityTerm{
// 						LabelSelector: &metav1.LabelSelector{
// 							MatchExpressions: []metav1.LabelSelectorRequirement{
// 								{
// 									Key:      p.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Key,
// 									Operator: p.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Operator,
// 									Values:   p.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.LabelSelector.MatchExpressions[0].Values,
// 								},
// 							},
// 						},
// 						TopologyKey: p.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey,
// 					},
// 				},
// 			}
// 			podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution = preferredDuringSchedulingIgnoredDuringExecution
// 		}
// 		affinity.PodAntiAffinity = podAntiAffinity
// 	}
// 	return affinity
// }

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
