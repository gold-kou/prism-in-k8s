package istio

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gold-kou/prism-in-k8s/app/params"
	"google.golang.org/protobuf/types/known/durationpb"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	"istio.io/client-go/pkg/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
)

const (
	defaultDelayNanos      = 100000000 // 100ms
	defaultDelayPercentage = 100.0     // 100%
)

func CreateIstioResources(ctx context.Context, kubeconfig *restclient.Config, namespaceName, resourceName string, routes []params.VirtualServiceRoute) error {
	// Istio clientset
	istioClientSet, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create Istio client: %w", err)
	}

	host := resourceName + "." + namespaceName + ".svc.cluster.local"
	httpRoutes := buildHTTPRoutes(host, routes)

	// VirtualService
	virtualService := &v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName,
		},
		Spec: networkingv1alpha3.VirtualService{
			Hosts: []string{host},
			Http:  httpRoutes,
		},
	}
	_, err = istioClientSet.NetworkingV1alpha3().VirtualServices(namespaceName).Create(ctx, virtualService, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create VirtualService: %w", err)
		}
		slog.Warn("The VirtualService already exists")
	} else {
		slog.Info("VirtualService is created successfully")
	}
	return nil
}

func buildHTTPRoutes(host string, routes []params.VirtualServiceRoute) []*networkingv1alpha3.HTTPRoute {
	// if route parameters are empty, default behavior
	if len(routes) == 0 {
		return []*networkingv1alpha3.HTTPRoute{
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
							FixedDelay: &durationpb.Duration{Nanos: int32(defaultDelayNanos)},
						},
					},
				},
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					{Destination: &networkingv1alpha3.Destination{Host: host}},
				},
			},
			{
				Name: "default",
				Route: []*networkingv1alpha3.HTTPRouteDestination{
					{Destination: &networkingv1alpha3.Destination{Host: host}},
				},
			},
		}
	}

	httpRoutes := make([]*networkingv1alpha3.HTTPRoute, 0, len(routes)+1)
	for _, route := range routes {
		httpRoute := &networkingv1alpha3.HTTPRoute{
			Name: route.Name,
			Route: []*networkingv1alpha3.HTTPRouteDestination{
				{Destination: &networkingv1alpha3.Destination{Host: host}},
			},
		}

		if route.URIPrefix != "" || route.Method != "" {
			match := &networkingv1alpha3.HTTPMatchRequest{}
			if route.URIPrefix != "" {
				match.Uri = &networkingv1alpha3.StringMatch{
					MatchType: &networkingv1alpha3.StringMatch_Prefix{Prefix: route.URIPrefix},
				}
			}
			if route.Method != "" {
				match.Method = &networkingv1alpha3.StringMatch{
					MatchType: &networkingv1alpha3.StringMatch_Exact{Exact: route.Method},
				}
			}
			httpRoute.Match = []*networkingv1alpha3.HTTPMatchRequest{match}
		}

		if route.DelayNanos > 0 {
			percentage := route.DelayPercentage
			if percentage == 0 {
				percentage = defaultDelayPercentage
			}
			httpRoute.Fault = &networkingv1alpha3.HTTPFaultInjection{
				Delay: &networkingv1alpha3.HTTPFaultInjection_Delay{
					Percentage: &networkingv1alpha3.Percent{Value: percentage},
					HttpDelayType: &networkingv1alpha3.HTTPFaultInjection_Delay_FixedDelay{
						FixedDelay: &durationpb.Duration{Nanos: route.DelayNanos},
					},
				},
			}
		}

		httpRoutes = append(httpRoutes, httpRoute)
	}

	// always add default catch-all route
	httpRoutes = append(httpRoutes, &networkingv1alpha3.HTTPRoute{
		Name: "default",
		Route: []*networkingv1alpha3.HTTPRouteDestination{
			{Destination: &networkingv1alpha3.Destination{Host: host}},
		},
	})

	return httpRoutes
}

func DeleteIstioResources(ctx context.Context, kubeconfig *restclient.Config, namespaceName, resourceName string) error {
	// Istio clientset
	istioClientSet, err := versioned.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create Istio client: %w", err)
	}
	slog.Info("Clientset of istio set up successfully")

	err = istioClientSet.NetworkingV1alpha3().VirtualServices(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete VirtualService: %w", err)
		}
		slog.Warn("The VirtualService is not found")
	} else {
		slog.Info("VirtualService is deleted successfully")
	}
	return nil
}
