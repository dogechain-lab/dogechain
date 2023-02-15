package awsssm

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/dogechain-lab/dogechain/secrets"

	"github.com/hashicorp/go-hclog"
)

// AwsSsmManager is a SecretsManager that
// stores secrets on AWS SSM Parameter Store
type AwsSsmManager struct {
	// Local logger object
	logger hclog.Logger

	// The AWS region
	region string

	// The AWS SSM client
	client *ssm.Client

	// The base path to store the secrets in SSM Parameter Store
	basePath string
}

// SecretsManagerFactory implements the factory method
func SecretsManagerFactory(
	config *secrets.SecretsManagerConfig,
	params *secrets.SecretsManagerParams) (secrets.SecretsManager, error) { //nolint

	// Check if the node name is present
	if config.Name == "" {
		return nil, errors.New("no node name specified for AWS SSM secrets manager")
	}

	// Check if the extra map is present
	if config.Extra == nil || config.Extra["region"] == nil || config.Extra["ssm-parameter-path"] == nil {
		return nil, errors.New("required extra map containing 'region' and 'ssm-parameter-path' not found for aws-ssm")
	}

	/// Set up the base object
	awsSsmManager := &AwsSsmManager{
		logger: params.Logger.Named(string(secrets.AWSSSM)),
		region: fmt.Sprintf("%v", config.Extra["region"]),
	}

	// Set the base path to store the secrets in SSM
	awsSsmManager.basePath = fmt.Sprintf("%s/%s", config.Extra["ssm-parameter-path"], config.Name)

	// Run the initial setup
	if err := awsSsmManager.Setup(); err != nil {
		return nil, err
	}

	return awsSsmManager, nil
}

// Setup sets up the AWS SSM secrets manager
func (a *AwsSsmManager) Setup() error {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithDefaultRegion(a.region),
	)

	if err != nil {
		return fmt.Errorf("unable to initialize AWS SSM client: %w", err)
	}

	ssmsvc := ssm.NewFromConfig(cfg)
	a.client = ssmsvc

	return nil
}

// constructSecretPath is a helper method for constructing a path to the secret
func (a *AwsSsmManager) constructSecretPath(name string) string {
	return fmt.Sprintf("%s/%s", a.basePath, name)
}

// GetSecret fetches a secret from AWS SSM
func (a *AwsSsmManager) GetSecret(name string) ([]byte, error) {
	param, err := a.client.GetParameter(
		context.TODO(),
		&ssm.GetParameterInput{
			Name:           aws.String(a.constructSecretPath(name)),
			WithDecryption: aws.Bool(true),
		})
	if err != nil || param == nil {
		return nil, secrets.ErrSecretNotFound
	}

	value := *param.Parameter.Value

	return []byte(value), nil
}

// SetSecret saves a secret to AWS SSM
func (a *AwsSsmManager) SetSecret(name string, value []byte) error {
	if _, err := a.client.PutParameter(
		context.TODO(),
		&ssm.PutParameterInput{
			Name:      aws.String(a.constructSecretPath(name)),
			Value:     aws.String(string(value)),
			Tier:      types.ParameterTierStandard,
			Type:      types.ParameterTypeSecureString,
			Overwrite: aws.Bool(false),
		}); err != nil {
		return fmt.Errorf("unable to store secret (%s), %w", name, err)
	}

	return nil
}

// HasSecret checks if the secret is present on AWS SSM ParameterStore
func (a *AwsSsmManager) HasSecret(name string) bool {
	_, err := a.GetSecret(name)

	return err == nil
}

// RemoveSecret removes a secret from AWS SSM ParameterStore
func (a *AwsSsmManager) RemoveSecret(name string) error {
	// Check if non-existent
	if _, err := a.GetSecret(name); err != nil {
		return err
	}

	// Delete the secret from SSM
	if _, err := a.client.DeleteParameter(
		context.TODO(),
		&ssm.DeleteParameterInput{
			Name: aws.String(a.constructSecretPath(name)),
		}); err != nil {
		return fmt.Errorf("unable to delete secret (%s), %w", name, err)
	}

	return nil
}
