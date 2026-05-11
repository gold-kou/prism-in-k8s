package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gold-kou/prism-in-k8s/app/istio"
	"github.com/gold-kou/prism-in-k8s/app/k8s"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/gold-kou/prism-in-k8s/app/registry"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type App struct {
	Config        *params.Config
	OpenAPIPath   string
	IsTest        bool
	AWSConfig     aws.Config
	AWSAccountID  string
	KubeConfig    *restclient.Config
	ResourceName  string
	NamespaceName string
}

func NewApp(ctx context.Context, cfg *params.Config, openapiPath string, isTest bool) (*App, error) {
	a := &App{
		Config:        cfg,
		OpenAPIPath:   openapiPath,
		IsTest:        isTest,
		ResourceName:  cfg.MicroserviceName + cfg.PrismMockSuffix,
		NamespaceName: cfg.MicroserviceNamespace + cfg.PrismMockSuffix,
	}

	// if test, don't load aws config
	if !isTest {
		awsCfg, err := config.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		stsClient := sts.NewFromConfig(awsCfg)
		result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return nil, fmt.Errorf("failed to get caller identity: %w", err)
		}
		a.AWSConfig = awsCfg
		a.AWSAccountID = *result.Account
	}

	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubeconfig: %w", err)
	}
	a.KubeConfig = kubeCfg

	return a, nil
}

func (a *App) Create(ctx context.Context) error {
	if !a.IsTest {
		if err := registry.BuildAndPushECR(ctx, a.Config, a.AWSConfig, a.AWSAccountID, a.ResourceName, a.OpenAPIPath); err != nil {
			return fmt.Errorf("ECR build/push failed: %w", err)
		}
	}

	if err := k8s.CreateK8sResources(ctx, a.Config, a.AWSAccountID, a.AWSConfig, a.KubeConfig, a.NamespaceName, a.ResourceName, a.IsTest); err != nil {
		return fmt.Errorf("k8s resource creation failed: %w", err)
	}

	if a.Config.IstioMode {
		if err := istio.CreateIstioResources(ctx, a.KubeConfig, a.NamespaceName, a.ResourceName, a.Config.VirtualServiceRoutes); err != nil {
			return fmt.Errorf("istio resource creation failed: %w", err)
		}
	}

	slog.Info("All resources for prism mock are created successfully")
	return nil
}

func (a *App) Delete(ctx context.Context) error {
	if a.Config.IstioMode {
		if err := istio.DeleteIstioResources(ctx, a.KubeConfig, a.NamespaceName, a.ResourceName); err != nil {
			return fmt.Errorf("istio resource deletion failed: %w", err)
		}
	}

	if err := k8s.DeleteK8sResources(ctx, a.KubeConfig, a.NamespaceName, a.ResourceName); err != nil {
		return fmt.Errorf("k8s resource deletion failed: %w", err)
	}

	if err := registry.DeleteECR(ctx, a.AWSConfig, a.ResourceName); err != nil {
		return fmt.Errorf("ECR deletion failed: %w", err)
	}

	slog.Info("All resources for prism mock are deleted successfully")
	return nil
}
