package secrets

import (
	"context"
	"errors"
)

type SecretsManagerClient struct{}

func NewSecretsManagerClient(context.Context, any) (*SecretsManagerClient, error) {
	return nil, errors.New("Cloud configuration is not available.")
}
func (c *SecretsManagerClient) Get(context.Context, string) (string, error) {
	return "", errors.New("Set this value in the environment.")
}
