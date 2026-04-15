package istio_test

import (
	"context"
	"testing"

	"github.com/gold-kou/prism-in-k8s/app/istio"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/gold-kou/prism-in-k8s/app/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCreateIstioResources(t *testing.T) {
	testNamespaceName := "test-namespace" + uuid.NewString()
	testResourceName := "test-resource" + uuid.NewString()

	ctx := context.TODO()
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err)

	// create namespace to create a virtualservice
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	require.NoError(t, err)
	err = testutil.CreateNamespace(ctx, k8sClientSet, testNamespaceName)
	require.NoError(t, err)

	// test target
	routes := []params.VirtualServiceRoute{
		{
			Name:            "example1",
			URIPrefix:       "/example1/",
			Method:          "GET",
			DelayNanos:      100000000,
			DelayPercentage: 100,
		},
		{
			Name:      "example2",
			URIPrefix: "/example2/",
			Method:    "POST",
		},
	}
	err = istio.CreateIstioResources(ctx, kubeconfig, testNamespaceName, testResourceName, routes)
	assert.NoError(t, err)

	// verify
	istioClient, err := versioned.NewForConfig(kubeconfig)
	assert.NoError(t, err)
	vs, err := istioClient.NetworkingV1alpha3().VirtualServices(testNamespaceName).Get(ctx, testResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	// verify configured routes + trailing default catch-all
	httpRoutes := vs.Spec.GetHttp()
	require.Len(t, httpRoutes, 3)
	assert.Equal(t, "example1", httpRoutes[0].GetName())
	assert.Equal(t, "/example1/", httpRoutes[0].GetMatch()[0].GetUri().GetPrefix())
	assert.Equal(t, "GET", httpRoutes[0].GetMatch()[0].GetMethod().GetExact())
	assert.Equal(t, int32(100000000), httpRoutes[0].GetFault().GetDelay().GetFixedDelay().GetNanos())
	assert.InDelta(t, 100.0, httpRoutes[0].GetFault().GetDelay().GetPercentage().GetValue(), 0.001)
	assert.Equal(t, "example2", httpRoutes[1].GetName())
	assert.Equal(t, "/example2/", httpRoutes[1].GetMatch()[0].GetUri().GetPrefix())
	assert.Equal(t, "POST", httpRoutes[1].GetMatch()[0].GetMethod().GetExact())
	assert.Nil(t, httpRoutes[1].GetFault())
	assert.Equal(t, "default", httpRoutes[2].GetName())

	// clean up
	err = testutil.DeleteNamespace(ctx, k8sClientSet, testNamespaceName)
	require.NoError(t, err)
}

func TestDeleteIstioResources(t *testing.T) {
	testNamespaceName := "test-namespace" + uuid.NewString()
	testResourceName := "test-resource" + uuid.NewString()

	ctx := context.TODO()
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err)
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	require.NoError(t, err)
	istioClientSet, err := versioned.NewForConfig(kubeconfig)
	require.NoError(t, err)

	// dummy resources
	err = testutil.CreateNamespace(ctx, k8sClientSet, testNamespaceName)
	require.NoError(t, err)
	err = testutil.CreateVirtualService(ctx, istioClientSet, testNamespaceName, testResourceName)
	require.NoError(t, err)

	// test target
	err = istio.DeleteIstioResources(ctx, kubeconfig, testNamespaceName, testResourceName)
	assert.NoError(t, err)

	// verify
	_, err = istioClientSet.NetworkingV1alpha3().VirtualServices(testNamespaceName).Get(ctx, testResourceName, metav1.GetOptions{})
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))

	// clean up
	err = testutil.DeleteNamespace(ctx, k8sClientSet, testNamespaceName)
	require.NoError(t, err)
}
