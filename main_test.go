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
		os:        &os,
		ssm:       c,
		batchSize: defaultBatchSize,
	}

	decrypt := false
	nofail := false
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_NoSSMParametersPrint(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        &os,
		ssm:       c,
		batchSize: defaultBatchSize,
	}

	decrypt := false
	nofail := false
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"TERM=screen-256color",
	}, os.Environ())
	assert.Equal(t, "", os.Stdout())

	c.AssertExpectations(t)
}

func TestExpandEnviron_SimpleSSMParameter(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        &os,
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
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET=hehe",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_SimpleSSMParameterPrint(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        &os,
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
	print := true
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET=ssm://secret",
		"TERM=screen-256color",
	}, os.Environ())
	assert.Equal(t, "SUPER_SECRET=hehe", os.Stdout())

	c.AssertExpectations(t)
}

func TestExpandEnviron_VersionedSSMParameter(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        &os,
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
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
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
		os:        &os,
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
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
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
		os:        &os,
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
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
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
		os:        &os,
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
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
	assert.Equal(t, &invalidParametersError{InvalidParameters: []string{"secret"}}, err)

	c.AssertExpectations(t)
}

func TestExpandEnviron_InvalidParametersNoFail(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        &os,
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
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)

	assert.NoError(t, err)
	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET=ssm://secret",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_BatchParameters(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        &os,
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
	print := false
	vars, err := e.expandEnviron(decrypt, nofail)
	e.setEnviron(print, vars)
	assert.NoError(t, err)

	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET_A=val-a",
		"SUPER_SECRET_B=val-b",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

type fakeEnviron struct {
	env    map[string]string
	stdout string
}

func newFakeEnviron() fakeEnviron {
	return fakeEnviron{
		env: map[string]string{
			"SHELL": "/bin/bash",
			"TERM":  "screen-256color",
		},
		stdout: "",
	}
}

func (e fakeEnviron) Environ() []string {
	var env sort.StringSlice
	for k, v := range e.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	env.Sort()
	return env
}

func (e fakeEnviron) Setenv(key, val string) {
	e.env[key] = val
}

func (e fakeEnviron) Getenv(key string) string {
	return e.env[key]
}

func (e *fakeEnviron) Write(s string) error {
	e.stdout += s

	return nil
}

func (e fakeEnviron) Stdout() string {
	return e.stdout
}

type mockSSM struct {
	mock.Mock
}

func (m *mockSSM) GetParameters(input *ssm.GetParametersInput) (*ssm.GetParametersOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*ssm.GetParametersOutput), args.Error(1)
}
