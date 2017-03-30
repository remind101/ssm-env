package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// Prefix is used to determine if an environment variable should be obtained
// from AWS Parameter Store.
const Prefix = "ssm://"

func main() {
	var (
		decrypt = flag.Bool("with-decryption", false, "Will attempt to decrypt the parameter, and set the env var as plaintext")
	)
	flag.Parse()
	args := flag.Args()

	path, err := exec.LookPath(args[0])
	must(err)

	var os osEnviron
	e := &expander{ssm: ssm.New(session.New()), os: os}
	must(e.expandEnviron(*decrypt))
	must(syscall.Exec(path, args[0:], os.Environ()))
}

type ssmClient interface {
	GetParameters(*ssm.GetParametersInput) (*ssm.GetParametersOutput, error)
}

type environ interface {
	Environ() []string
	Setenv(key, vale string)
}

type osEnviron int

func (e osEnviron) Environ() []string {
	return os.Environ()
}

func (e osEnviron) Setenv(key, val string) {
	os.Setenv(key, val)
}

type ssmVar struct {
	envvar    string
	parameter string
}

type expander struct {
	ssm ssmClient
	os  environ
}

func (e *expander) expandEnviron(decrypt bool) error {
	// Environment variables that point to some SSM parameters.
	var ssmVars []ssmVar

	input := &ssm.GetParametersInput{
		WithDecryption: aws.Bool(decrypt),
	}

	names := make(map[string]bool)
	for _, envvar := range e.os.Environ() {
		k, v := splitVar(envvar)
		if strings.HasPrefix(v, Prefix) {
			// The name of the SSM parameter.
			parameter := v[len(Prefix):]
			names[parameter] = true
			ssmVars = append(ssmVars, ssmVar{k, parameter})
		}
	}

	for k := range names {
		input.Names = append(input.Names, aws.String(k))
	}

	if len(input.Names) == 0 {
		// Nothing to do, no SSM parameters.
		return nil
	}

	resp, err := e.ssm.GetParameters(input)
	if err != nil {
		return err
	}

	if len(resp.InvalidParameters) > 0 {
		return newInvalidParametersError(resp)
	}

	values := make(map[string]string)
	for _, p := range resp.Parameters {
		values[*p.Name] = *p.Value
	}

	for _, v := range ssmVars {
		e.os.Setenv(v.envvar, values[v.parameter])
	}

	return nil
}

type invalidParametersError struct {
	InvalidParameters []string
}

func newInvalidParametersError(resp *ssm.GetParametersOutput) *invalidParametersError {
	e := new(invalidParametersError)
	for _, p := range resp.InvalidParameters {
		if p == nil {
			continue
		}

		e.InvalidParameters = append(e.InvalidParameters, *p)
	}
	return e
}

func (e *invalidParametersError) Error() string {
	return fmt.Sprintf("invalid parameters: %v", e.InvalidParameters)
}

func splitVar(v string) (key, val string) {
	parts := strings.Split(v, "=")
	return parts[0], parts[1]
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ssm-env: %v\n", err)
		os.Exit(1)
	}
}
