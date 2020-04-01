package main

import (
	"fmt"
	"sort"
	"testing"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExpandEnviron_NoSSMParameters(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		batchSize: defaultBatchSize,
	}

	decrypt := false
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
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
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		batchSize: defaultBatchSize,
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
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET=hehe",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_VersionedSSMParameter(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm://secret:1")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("secret:1")},
		WithDecryption: aws.Bool(true),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("secret"), Value: aws.String("versioned"), Selector: aws.String(":1")},
		},
	}, nil)

	decrypt := true
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET=versioned",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_CustomTemplate(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(`{{ if eq .Name "SUPER_SECRET" }}secret{{end}}`)),
		os:        os,
		ssm:       c,
		batchSize: defaultBatchSize,
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
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
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
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		batchSize: defaultBatchSize,
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
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
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
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm://secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("secret")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		InvalidParameters: []*string{aws.String("secret")},
	}, nil)

	decrypt := false
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.Equal(t, &invalidParametersError{InvalidParameters: []string{"secret"}}, err)

	c.AssertExpectations(t)
}

func TestExpandEnviron_InvalidParametersNoFail(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm://secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("secret")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		InvalidParameters: []*string{aws.String("secret")},
	}, nil)

	decrypt := false
	nofail := true
	err := e.expandEnviron(decrypt, nofail)

  assert.NoError(t, err)
	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET=",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_BatchParameters(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		batchSize: 1,
	}

	os.Setenv("SUPER_SECRET_A", "ssm://secret-a")
	os.Setenv("SUPER_SECRET_B", "ssm://secret-b")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("secret-a")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("secret-a"), Value: aws.String("val-a")},
		},
	}, nil)

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("secret-b")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("secret-b"), Value: aws.String("val-b")},
		},
	}, nil)

	decrypt := false
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET_A=val-a",
		"SUPER_SECRET_B=val-b",
		"TERM=screen-256color",
	}, os.Environ())

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
