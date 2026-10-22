package parameterstore

import (
	"context"
	"errors"
)

type ParameterStore struct{}

func NewParameterStore(context.Context, any) (*ParameterStore, error) {
	return nil, errors.New("Cloud configuration is not available.")
}
func (c *ParameterStore) Get(context.Context, string) (string, error) {
	return "", errors.New("Set this value in the environment.")
}
