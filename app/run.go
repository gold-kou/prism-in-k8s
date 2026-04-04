package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gold-kou/prism-in-k8s/app/istio"
	"github.com/gold-kou/prism-in-k8s/app/k8s"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/gold-kou/prism-in-k8s/app/registry"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	errFailedToLoadAWSConfig     = errors.New("failed to load AWS config")
	errFailedToGetCallerIdentity = errors.New("failed to get caller identity")
	errFailedToBuildKubeConfig   = errors.New("failed to build Kubeconfig")
	errParamsConfigPathNotSet    = errors.New("PARAMS_CONFIG_PATH is not set")
)

type App struct {
	config        *params.Config
	awsConfig     aws.Config
	awsAccountID  string
	kubeConfig    *restclient.Config
	resourceName  string
	namespaceName string
	isCreate      bool
	isDelete      bool
	isTest        bool
}

func NewApp() (*App, error) {
	application := &App{}

	flag.BoolVar(&application.isCreate, "create", false, "set to true if running in create mode")
	flag.BoolVar(&application.isDelete, "delete", false, "set to true if running in delete mode")
	flag.BoolVar(&application.isTest, "test", false, "set to true if running in test mode")
	flag.Parse()

	// load and validate config
	configPath := os.Getenv("PARAMS_CONFIG_PATH")
	if configPath == "" {
		return nil, errParamsConfigPathNotSet
	}

	cfg, err := params.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	application.config = cfg

	if !application.isTest {
		// AWS config
		application.awsConfig, err = awsconfig.LoadDefaultConfig(context.Background())
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errFailedToLoadAWSConfig, err)
		}

		// get AWS account ID
		stsClient := sts.NewFromConfig(application.awsConfig)
		result, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errFailedToGetCallerIdentity, err)
		}
		application.awsAccountID = *result.Account
	}

	// kube config
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	application.kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errFailedToBuildKubeConfig, err)
	}

	// resource name
	application.resourceName = "test-microservice"
	application.namespaceName = "test-namespace"
	if cfg.MicroserviceName != "" && cfg.MicroserviceNamespace != "" {
		application.resourceName = cfg.MicroserviceName + cfg.PrismMockSuffix
		application.namespaceName = cfg.MicroserviceNamespace + cfg.PrismMockSuffix
	}

	return application, nil
}

func (a *App) Run() {
	ctx, cancel := context.WithTimeout(context.Background(), a.config.Timeout)
	defer cancel()

	if a.isCreate {
		if !a.isTest {
			err := registry.BuildAndPushECR(ctx, a.awsConfig, a.awsAccountID, a.resourceName, a.config)
			if err != nil {
				panic(err)
			}
		}

		err := k8s.CreateK8sResources(ctx, a.awsAccountID, a.awsConfig, a.kubeConfig, a.namespaceName, a.resourceName, a.config, a.isTest)
		if err != nil {
			panic(err)
		}

		if a.config.IstioMode {
			err = istio.CreateIstioResources(ctx, a.kubeConfig, a.namespaceName, a.resourceName)
			if err != nil {
				panic(err)
			}
		}
		log.Println("[INFO] All resources for prism mock are created successfully")
	} else if a.isDelete {
		if a.config.IstioMode {
			err := istio.DeleteIstioResources(ctx, a.kubeConfig, a.namespaceName, a.resourceName)
			if err != nil {
				panic(err)
			}
		}

		err := k8s.DeleteK8sResources(ctx, a.kubeConfig, a.namespaceName, a.resourceName)
		if err != nil {
			panic(err)
		}

		err = registry.DeleteECR(ctx, a.awsConfig, a.resourceName)
		if err != nil {
			panic(err)
		}
		log.Println("[INFO] All resources for prism mock are deleted successfully")
	}
}
