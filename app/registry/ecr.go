package registry

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/gold-kou/prism-in-k8s/app/params"
)

func BuildAndPushECR(ctx context.Context, awsConfig aws.Config, awsAccountID, resourceName, openapiPath string) error {
	// build Docker image using a temporary build context
	buildCtx, err := prepareBuildContext(openapiPath)
	if err != nil {
		return fmt.Errorf("failed to prepare docker build context: %w", err)
	}
	defer func() { _ = os.RemoveAll(buildCtx) }()

	imageTag := params.MicroserviceName + ":v1"
	cmd := exec.CommandContext(ctx, "docker", "build", "--platform", "linux/amd64", "-t", imageTag, buildCtx)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build docker image: %w", err)
	}
	slog.Info("Docker image is built successfully")

	// ECR tags
	tags := []types.Tag{}
	for _, ecrTag := range params.EcrTags {
		if ecrTag.Key != "" || ecrTag.Value != "" {
			tags = append(tags, types.Tag{
				Key:   aws.String(ecrTag.Key),
				Value: aws.String(ecrTag.Value),
			})
		}
	}

	// create ECR repository
	ecrClient := ecr.NewFromConfig(awsConfig)
	repositoryName := resourceName
	input := &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repositoryName),
		Tags:           tags,
	}
	_, err = ecrClient.CreateRepository(ctx, input)
	if err != nil {
		var ecrExistsException *types.RepositoryAlreadyExistsException
		if !errors.As(err, &ecrExistsException) {
			return fmt.Errorf("failed to create ECR repository: %w", err)
		}
		slog.Warn("The ECR already exists")
	} else {
		slog.Info("ECR is created successfully")
	}

	// tag Docker image for ECR
	ecrImageTag := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, awsConfig.Region, repositoryName)
	cmdTag := exec.CommandContext(ctx, "docker", "tag", imageTag, ecrImageTag)
	if err := cmdTag.Run(); err != nil {
		return fmt.Errorf("failed to tag image: %w", err)
	}
	slog.Info("Docker image tagged successfully")

	// login to ECR
	err = loginToECR(ctx, awsConfig, awsAccountID)
	if err != nil {
		return fmt.Errorf("failed to log in ECR: %w", err)
	}
	slog.Info("Logged in ECR successfully")

	// push image to ECR
	cmdPush := exec.CommandContext(ctx, "docker", "push", ecrImageTag)
	if err := cmdPush.Run(); err != nil {
		return fmt.Errorf("failed to push image to ECR: %w", err)
	}
	slog.Info("Docker image is pushed to ECR successfully")
	return nil
}

func loginToECR(ctx context.Context, awsConfig aws.Config, awsAccountID string) error {
	ecrClient := ecr.NewFromConfig(awsConfig)

	// Get the authorization token
	authTokenOutput, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{
		RegistryIds: []string{awsAccountID},
	})
	if err != nil {
		return fmt.Errorf("failed to log in ECR: %w", err)
	}

	if len(authTokenOutput.AuthorizationData) == 0 {
		return errors.New("failed to log in ECR: no authorization data found")
	}

	authData := authTokenOutput.AuthorizationData[0]
	decodedToken, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return fmt.Errorf("failed to log in ECR: %w", err)
	}

	decodedTokenParts := 2
	parts := strings.SplitN(string(decodedToken), ":", decodedTokenParts)
	if len(parts) != decodedTokenParts {
		return errors.New("failed to log in ECR: invalid authorization token format")
	}

	username := parts[0]
	password := parts[1]
	registry := *authData.ProxyEndpoint

	loginCmd := exec.CommandContext(ctx, "docker", "login", "--username", username, "--password-stdin", registry)
	loginCmd.Stdin = strings.NewReader(password)
	output, err := loginCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to log in ECR: %w\n%s", err, string(output))
	}

	return nil
}

func DeleteECR(ctx context.Context, awsConfig aws.Config, resourceName string) error {
	// Delete ECR
	ecrClient := ecr.NewFromConfig(awsConfig)
	repositoryName := resourceName
	input := &ecr.DeleteRepositoryInput{
		RepositoryName: aws.String(repositoryName),
		Force:          true, // Force delete to remove all images
	}
	_, err := ecrClient.DeleteRepository(ctx, input)
	if err != nil {
		var ecrNotFoundException *types.RepositoryNotFoundException
		if !errors.As(err, &ecrNotFoundException) {
			return fmt.Errorf("failed to delete ECR repository: %w", err)
		}
		slog.Warn("The ECR is not found")
	} else {
		slog.Info("ECR is deleted successfully")
	}
	return nil
}
