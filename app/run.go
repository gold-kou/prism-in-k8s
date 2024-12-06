package app

import (
	"context"
	"flag"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gold-kou/prism-in-k8s/app/istio"
	"github.com/gold-kou/prism-in-k8s/app/k8s"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/gold-kou/prism-in-k8s/app/registry"
	"golang.org/x/xerrors"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	isCreate      bool
	isDelete      bool
	isTest        bool
	awsConfig     aws.Config
	awsAccountID  string
	kubeConfig    *restclient.Config
	resourceName  string
	namespaceName string
)

func init() {
	// command args
	flag.BoolVar(&isCreate, "create", false, "set to true if running in create mode")
	flag.BoolVar(&isDelete, "delete", false, "set to true if running in delete mode")
	flag.BoolVar(&isTest, "test", false, "set to true if running in test mode")
	flag.Parse()

	// validation parameters
	err := params.ValidateParams()
	if err != nil {
		panic(err)
	}

	if !isTest {
		// AWS config
		awsConfig, err = config.LoadDefaultConfig(context.Background())
		if err != nil {
			panic(xerrors.Errorf("failed load AWS config: %v", err))
		}

		// get AWS account ID
		stsClient := sts.NewFromConfig(awsConfig)
		result, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
		if err != nil {
			panic(xerrors.Errorf("failed to get caller identity: %v", err))
		}
		awsAccountID = *result.Account
	}

	// kube config
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(xerrors.Errorf("failed to build Kubeconfig: %v", err))
	}

	// resource name
	resourceName = "test-microservice"
	namespaceName = "test-namespace"
	if params.MicroserviceName != "" && params.MicroserviceNamespace != "" {
		resourceName = params.MicroserviceName + params.PrismMockSuffix
		namespaceName = params.MicroserviceNamespace + params.PrismMockSuffix
	}
}

func Run() {
	ctx, cancel := context.WithTimeout(context.Background(), params.Timeout)
	defer cancel()

	if isCreate {
		if !isTest {
			err := registry.BuildAndPushECR(ctx, awsConfig, awsAccountID, resourceName)
			if err != nil {
				panic(err)
			}
		}

		err := k8s.CreateK8sResources(ctx, awsAccountID, awsConfig, kubeConfig, namespaceName, resourceName, params.IstioMode, isTest)
		if err != nil {
			panic(err)
		}

		if params.IstioMode {
			err = istio.CreateIstioResources(ctx, kubeConfig, namespaceName, resourceName)
			if err != nil {
				panic(err)
			}
		}
		log.Println("[INFO] All resources for prism mock are created successfully")
	} else if isDelete {
		if params.IstioMode {
			err := istio.DeleteIstioResources(ctx, kubeConfig, namespaceName, resourceName)
			if err != nil {
				panic(err)
			}
		}

		err := k8s.DeleteK8sResources(ctx, kubeConfig, namespaceName, resourceName)
		if err != nil {
			panic(err)
		}

		err = registry.DeleteECR(ctx, awsConfig, resourceName)
		if err != nil {
			panic(err)
		}
		log.Println("[INFO] All resources for prism mock are deleted successfully")
	}
}
