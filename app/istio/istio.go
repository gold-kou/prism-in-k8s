package istio

import (
	"context"
	"log"

	"github.com/golang/protobuf/ptypes/duration"
	"github.com/pingcap/errors"
	"golang.org/x/xerrors"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	"istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
)

const (
	defaultDelayNanos      = 100000000 // 100ms
	defaultDelayPercentage = 100.0     // 100%
)

var (
	errFailedToCreateIstioClient    = errors.New("failed to create Istio client")
	errFailedToCreateVirtualService = errors.New("failed to create VirtualService")
	errFailedToDeleteVirtualService = errors.New("failed to delete VirtualService")
)

func CreateIstioResources(ctx context.Context, kubeconfig *restclient.Config, namespaceName, resourceName string) error {
	// Istio clientset
	istioClientSet, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToCreateIstioClient, err)
	}

	// VirtualService
	virtualService := &v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName,
		},
		Spec: networkingv1alpha3.VirtualService{
			Hosts: []string{
				resourceName + "." + namespaceName + ".svc.cluster.local",
			},
			Http: []*networkingv1alpha3.HTTPRoute{
				{
					Name: "example1",
					Match: []*networkingv1alpha3.HTTPMatchRequest{
						{
							Uri: &networkingv1alpha3.StringMatch{
								MatchType: &networkingv1alpha3.StringMatch_Prefix{
									Prefix: "/example1/",
								},
							},
							Method: &networkingv1alpha3.StringMatch{
								MatchType: &networkingv1alpha3.StringMatch_Exact{
									Exact: "GET",
								},
							},
						},
					},
					Fault: &networkingv1alpha3.HTTPFaultInjection{
						Delay: &networkingv1alpha3.HTTPFaultInjection_Delay{
							Percentage: &networkingv1alpha3.Percent{
								Value: defaultDelayPercentage,
							},
							HttpDelayType: &networkingv1alpha3.HTTPFaultInjection_Delay_FixedDelay{
								FixedDelay: &duration.Duration{Nanos: int32(defaultDelayNanos)}, // 100ms
							},
						},
					},
					Route: []*networkingv1alpha3.HTTPRouteDestination{
						{
							Destination: &networkingv1alpha3.Destination{
								Host: resourceName + "." + namespaceName + ".svc.cluster.local",
							},
						},
					},
				},
				{
					Name: "default",
					Route: []*networkingv1alpha3.HTTPRouteDestination{
						{
							Destination: &networkingv1alpha3.Destination{
								Host: resourceName + "." + namespaceName + ".svc.cluster.local",
							},
						},
					},
				},
			},
		},
	}
	_, err = istioClientSet.NetworkingV1alpha3().VirtualServices(namespaceName).Create(ctx, virtualService, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return xerrors.Errorf("%w: %w", errFailedToCreateVirtualService, err)
		}
		log.Println("[WARN] The VirtualService already exists")
	} else {
		log.Println("[INFO] VirtualService is created successfully")
	}
	return nil
}

func DeleteIstioResources(ctx context.Context, kubeconfig *restclient.Config, namespaceName, resourceName string) error {
	// Istio clientset
	istioClientSet, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToCreateIstioClient, err)
	}
	log.Println("[INFO] Clientset of istio set up successfully")

	err = istioClientSet.NetworkingV1alpha3().VirtualServices(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return xerrors.Errorf("%w: %w", errFailedToDeleteVirtualService, err)
		}
		log.Println("[WARN] The VirtualService is not found")
	} else {
		log.Println("[INFO] VirtualService is deleted successfully")
	}
	return nil
}
