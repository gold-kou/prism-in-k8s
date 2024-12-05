package testutil

import (
	"context"
	"errors"
	"time"

	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/gold-kou/prism-in-k8s/app/util"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	"istio.io/client-go/pkg/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const servicePort = 80

var errNotRunning = errors.New("pod did not reach Running state")

func CreateNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	n := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(ctx, n, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func CreateDeployment(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: util.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
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
							Name:  name,
							Image: "my-local-image:v1",
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
				},
			},
		},
	}
	_, err := clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func CreateService(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": name,
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
	_, err := clientset.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func CreateVirtualService(ctx context.Context, istioClientSet *versioned.Clientset, namespace, name string) error {
	virtualService := &v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: networkingv1alpha3.VirtualService{
			Hosts: []string{
				name + "." + namespace + ".svc.cluster.local",
			},
			Http: []*networkingv1alpha3.HTTPRoute{
				{
					Name: "default",
					Route: []*networkingv1alpha3.HTTPRouteDestination{
						{
							Destination: &networkingv1alpha3.Destination{
								Host: name + "." + namespace + ".svc.cluster.local",
							},
						},
					},
				},
			},
		},
	}
	_, err := istioClientSet.NetworkingV1alpha3().VirtualServices(namespace).Create(ctx, virtualService, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func DeleteNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	err := clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func DeleteDeployment(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) error {
	err := clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func DeleteService(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) error {
	err := clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func WaitForPodRunning(ctx context.Context, clientset *kubernetes.Clientset, namespace, resourceName string) error {
	label := "app=" + resourceName
	for range 30 {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: label,
		})
		if err != nil {
			return err
		}
		if len(pods.Items) > 0 {
			if pods.Items[0].Status.Phase == corev1.PodRunning {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return errNotRunning
}
