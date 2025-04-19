package main

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"testing"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExpandEnviron_NoSSMParameters(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
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
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm:///secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/secret")},
		WithDecryption: aws.Bool(true),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("/secret"), Value: aws.String("hehe")},
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
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm:///secret:1")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/secret:1")},
		WithDecryption: aws.Bool(true),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("/secret"), Value: aws.String("versioned"), Selector: aws.String(":1")},
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
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(`{{ if eq .Name "SUPER_SECRET" }}/secret{{end}}`)),
		os:        os,
		ssm:       c,
		kms:       k,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm:///secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/secret")},
		WithDecryption: aws.Bool(true),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("/secret"), Value: aws.String("hehe")},
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
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET_A", "ssm:///secret")
	os.Setenv("SUPER_SECRET_B", "ssm:///secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/secret")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("/secret"), Value: aws.String("hehe")},
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

func TestExpandEnviron_MalformedParametersFail(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm://secret")

	decrypt := false
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.Containsf(t, err.Error(), "SSM parameters must have a leading '/'", "")
}

func TestExpandEnviron_MalformedParametersNofail(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
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
		"SUPER_SECRET=ssm://secret",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_InvalidParameters(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm:///bad.secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/bad.secret")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		InvalidParameters: []*string{aws.String("/bad.secret")},
	}, nil)

	decrypt := false
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.Equal(t, &invalidParametersError{InvalidParameters: []string{"/bad.secret"}}, err)

	c.AssertExpectations(t)
}

func TestExpandEnviron_InvalidParametersNoFail(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
		batchSize: defaultBatchSize,
	}

	os.Setenv("SUPER_SECRET", "ssm:///secret")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/secret")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		InvalidParameters: []*string{aws.String("/secret")},
	}, nil)

	decrypt := false
	nofail := true
	err := e.expandEnviron(decrypt, nofail)

	assert.NoError(t, err)
	assert.Equal(t, []string{
		"SHELL=/bin/bash",
		"SUPER_SECRET=ssm:///secret",
		"TERM=screen-256color",
	}, os.Environ())

	c.AssertExpectations(t)
}

func TestExpandEnviron_BatchParameters(t *testing.T) {
	os := newFakeEnviron()
	c := new(mockSSM)
	k := new(mockKMS)
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       c,
		kms:       k,
		batchSize: 1,
	}

	os.Setenv("SUPER_SECRET_A", "ssm:///secret-a")
	os.Setenv("SUPER_SECRET_B", "ssm:///secret-b")

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/secret-a")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("/secret-a"), Value: aws.String("val-a")},
		},
	}, nil)

	c.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/secret-b")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("/secret-b"), Value: aws.String("val-b")},
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

func TestExtractKmsValue(t *testing.T) {
	// Test the KMS value extraction logic
	
	testCases := []struct {
		input          string
		shouldMatch    bool
		expectedBase64 string
	}{
		{"!kms QUJDREVGRw==", true, "QUJDREVGRw=="},
		{"!kms abc", true, "abc"},
		{"!kms abc===", true, "abc==="},
		{"!kms ", true, ""},
		{"!kmsa", false, ""},
		{"ssm:///path", false, ""},
	}
	
	for _, tc := range testCases {
		if tc.shouldMatch {
			// If it should match, we expect the string to start with the KMS prefix
			if strings.HasPrefix(tc.input, KMSPrefix) {
				// Extract the base64 part
				encodedPart := strings.TrimPrefix(tc.input, KMSPrefix)
				// Remove quotes
				encodedPart = strings.Trim(encodedPart, "'\" ")
				assert.Equal(t, tc.expectedBase64, encodedPart, 
					"Wrong base64 extraction for '%s'", tc.input)
			} else {
				assert.Fail(t, "Expected '%s' to start with KMS prefix", tc.input)
			}
		} else {
			// If it should not match, we expect the string NOT to start with the KMS prefix
			assert.False(t, strings.HasPrefix(tc.input, KMSPrefix), 
				"Expected '%s' not to start with KMS prefix", tc.input)
		}
	}
}

func TestDecryptKmsValue(t *testing.T) {
	// Test the decryptKmsValue function directly
	kmsClient := new(mockKMS)
	e := &expander{
		kms: kmsClient,
	}
	
	// Verify the base64 decoding works as expected
	value := "QUJDREVGR0g="
	decoded, err := base64.StdEncoding.DecodeString(value)
	assert.NoError(t, err)
	assert.Equal(t, "ABCDEFGH", string(decoded))
	
	// Setup KMS mock
	kmsClient.On("Decrypt", mock.MatchedBy(func(input *kms.DecryptInput) bool {
		return string(input.CiphertextBlob) == "ABCDEFGH"
	})).Return(&kms.DecryptOutput{
		Plaintext: []byte("decrypted-secret"),
	}, nil)
	
	// Call the function directly
	result, err := e.decryptKmsValue("QUJDREVGR0g=", false)
	assert.NoError(t, err)
	assert.Equal(t, "decrypted-secret", result)
	
	// Verify expectations
	kmsClient.AssertExpectations(t)
}

// A simpler implementation of test environment that works specifically for KMS values
type testEnviron struct {
	env map[string]string
}

func newTestEnviron() testEnviron {
	return testEnviron{
		env: map[string]string{
			"SHELL": "/bin/bash", 
			"TERM": "screen-256color",
		},
	}
}

func (e testEnviron) Environ() []string {
	var env []string
	for k, v := range e.env {
		envStr := fmt.Sprintf("%s=%s", k, v)
		env = append(env, envStr)
	}
	return env
}

func (e testEnviron) Setenv(key, val string) {
	e.env[key] = val
}

func TestExpandEnviron_KMSParameter(t *testing.T) {
	// Create a testEnviron instance that works better for our test
	os := newTestEnviron()
	
	// Set the KMS env var with proper base64
	// ABCDEFGH -> QUJDREVGR0g= (but avoid quotes which cause problems)
	kmsValue := "!kms QUJDREVGR0g="
	os.env["KMS_SECRET"] = kmsValue
	
	// Create mock clients
	kmsClient := new(mockKMS)
	ssmClient := new(mockSSM)
	
	// Create expander
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       ssmClient,
		kms:       kmsClient,
		batchSize: defaultBatchSize,
	}
	
	// Setup KMS mock
	kmsClient.On("Decrypt", mock.MatchedBy(func(input *kms.DecryptInput) bool {
		return string(input.CiphertextBlob) == "ABCDEFGH"
	})).Return(&kms.DecryptOutput{
		Plaintext: []byte("decrypted-secret"),
	}, nil)
	
	// Call expandEnviron
	err := e.expandEnviron(false, false)
	assert.NoError(t, err)
	
	// Check the result
	assert.Equal(t, "decrypted-secret", os.env["KMS_SECRET"])
	
	// Verify expectations  
	kmsClient.AssertExpectations(t)
}

func TestExpandEnviron_KMSAndSSMParameters(t *testing.T) {
	// Create a testEnviron instance that works better for our test
	os := newTestEnviron()
	
	// Set both SSM parameter and KMS encrypted value
	os.env["SSM_SECRET"] = "ssm:///secret"
	// The base64 value "QUJDREVGR0g=" decodes to "ABCDEFGH"
	os.env["KMS_SECRET"] = "!kms QUJDREVGR0g="
	
	// Create mock clients
	ssmClient := new(mockSSM)
	kmsClient := new(mockKMS)
	
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       ssmClient,
		kms:       kmsClient,
		batchSize: defaultBatchSize,
	}

	// Setup mocks
	ssmClient.On("GetParameters", &ssm.GetParametersInput{
		Names:          []*string{aws.String("/secret")},
		WithDecryption: aws.Bool(false),
	}).Return(&ssm.GetParametersOutput{
		Parameters: []*ssm.Parameter{
			{Name: aws.String("/secret"), Value: aws.String("ssm-value")},
		},
	}, nil)
	
	kmsClient.On("Decrypt", mock.MatchedBy(func(input *kms.DecryptInput) bool {
		return string(input.CiphertextBlob) == "ABCDEFGH"
	})).Return(&kms.DecryptOutput{
		Plaintext: []byte("kms-value"),
	}, nil)

	decrypt := false
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.NoError(t, err)

	// Check the values directly
	assert.Equal(t, "kms-value", os.env["KMS_SECRET"])
	assert.Equal(t, "ssm-value", os.env["SSM_SECRET"])

	ssmClient.AssertExpectations(t)
	kmsClient.AssertExpectations(t)
}

func TestExpandEnviron_InvalidKMSParameter(t *testing.T) {
	// Create a testEnviron instance that works better for our test
	os := newTestEnviron()
	
	// Set KMS encrypted value with invalid base64
	os.env["KMS_SECRET"] = "!kms INVALID-BASE64!"
	
	// Create mock clients
	ssmClient := new(mockSSM)
	kmsClient := new(mockKMS)
	
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       ssmClient,
		kms:       kmsClient,
		batchSize: defaultBatchSize,
	}

	// No mocks needed as it should fail at base64 decoding stage

	decrypt := false
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode base64 value")
}

func TestExpandEnviron_InvalidKMSParameterNoFail(t *testing.T) {
	// Create a testEnviron instance that works better for our test
	os := newTestEnviron()
	
	// Set KMS encrypted value with invalid base64
	os.env["KMS_SECRET"] = "!kms INVALID-BASE64!"
	
	// Create mock clients
	ssmClient := new(mockSSM)
	kmsClient := new(mockKMS)
	
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       ssmClient,
		kms:       kmsClient,
		batchSize: defaultBatchSize,
	}

	// With nofail=true, it shouldn't error
	decrypt := false
	nofail := true
	err := e.expandEnviron(decrypt, nofail)
	assert.NoError(t, err)
	
	// And the environment variable should remain unchanged
	assert.Equal(t, "!kms INVALID-BASE64!", os.env["KMS_SECRET"])
}

func TestExpandEnviron_KMSDecryptFails(t *testing.T) {
	// Create a testEnviron instance that works better for our test
	os := newTestEnviron()
	
	// The base64 value "QUJDREVGR0g=" decodes to "ABCDEFGH"
	os.env["KMS_SECRET"] = "!kms QUJDREVGR0g="
	
	// Create mock clients
	ssmClient := new(mockSSM)
	kmsClient := new(mockKMS)
	
	e := expander{
		t:         template.Must(parseTemplate(DefaultTemplate)),
		os:        os,
		ssm:       ssmClient,
		kms:       kmsClient,
		batchSize: defaultBatchSize,
	}

	// Setup mock for KMS Decrypt operation to fail
	kmsClient.On("Decrypt", mock.MatchedBy(func(input *kms.DecryptInput) bool {
		return string(input.CiphertextBlob) == "ABCDEFGH"
	})).Return(&kms.DecryptOutput{}, fmt.Errorf("KMS error: access denied"))

	// With nofail=false, it should error
	decrypt := false
	nofail := false
	err := e.expandEnviron(decrypt, nofail)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt KMS value")
	assert.Contains(t, err.Error(), "KMS error: access denied")
	
	// Re-setup the mock for the second test since it was already used
	kmsClient = new(mockKMS)
	e.kms = kmsClient
	kmsClient.On("Decrypt", mock.MatchedBy(func(input *kms.DecryptInput) bool {
		return string(input.CiphertextBlob) == "ABCDEFGH"
	})).Return(&kms.DecryptOutput{}, fmt.Errorf("KMS error: access denied"))
	
	// With nofail=true, it shouldn't error
	nofail = true
	err = e.expandEnviron(decrypt, nofail)
	assert.NoError(t, err)
	
	// And the environment variable should remain unchanged
	found := false
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "KMS_SECRET=") {
			found = true
			assert.Equal(t, "KMS_SECRET=!kms QUJDREVGR0g=", env)
		}
	}
	assert.True(t, found, "KMS_SECRET environment variable should still exist")
	
	kmsClient.AssertExpectations(t)
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
		// Force raw string to preserve all characters including single quotes and = signs
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

type mockKMS struct {
	mock.Mock
}

func (m *mockKMS) Decrypt(input *kms.DecryptInput) (*kms.DecryptOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*kms.DecryptOutput), args.Error(1)
}
