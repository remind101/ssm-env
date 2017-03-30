package main

import (
	"fmt"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExpandEnviron_NoSSMParameters(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		os:  os,
		ssm: c,
	}

	decrypt := false
	err := e.expandEnviron(decrypt)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_SimpleSSMParameter(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		os:  os,
		ssm: c,
	}

	os.Setenv("SUPER_SECRET", "ssm://secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("secret")},
		WithDecryption: aws.Bool(true),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("secret"), Value: aws.String("hehe")},
		},
	}, nil)

	decrypt := true
	err := e.expandEnviron(decrypt)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET=hehe",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_DuplicateSSMParameter(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		os:  os,
		ssm: c,
	}

	os.Setenv("SUPER_SECRET_A", "ssm://secret")
	os.Setenv("SUPER_SECRET_B", "ssm://secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("secret")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("secret"), Value: aws.String("hehe")},
		},
	}, nil)

	decrypt := false
	err := e.expandEnviron(decrypt)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET_A=hehe",
		"SUPER_SECRET_B=hehe",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_InvalidParameters(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		os:  os,
		ssm: c,
	}

	os.Setenv("SUPER_SECRET", "ssm://secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("secret")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		InvalidParameters: []*string{aws.String("secret")},
	}, nil)

	decrypt := false
	err := e.expandEnviron(decrypt)
	assert.Equal(t, &invalidParametersError{InvalidParameters: []string{"secret"}}, err)

	c.AssertExpectations(t)
}

type fakeEnviron map[string]string

func newFakeEnviron() fakeEnviron {
	return fakeEnviron{
		"SHELL": "/bin/bash",
		"TERM":  "screen-256color",
	}
}

func (e fakeEnviron) Environ() []string {
	var env sort.StringSlice
	for k, v := range e {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	env.Sort()
	return env
}

func (e fakeEnviron) Setenv(key, val string) {
	e[key] = val
}

type mockSSM struct {
	mock.Mock
}

func (m *mockSSM) GetParameters(input *ssm.GetParametersInput) (*ssm.GetParametersOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*ssm.GetParametersOutput), args.Error(1)
}
