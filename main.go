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

var client = ssm.New(session.New())

func main() {
	var (
		decrypt = flag.Bool("with-decryption", false, "Will attempt to decrypt the parameter, and set the env var as plaintext")
	)
	flag.Parse()
	args := flag.Args()

	path, err := exec.LookPath(args[0])
	must(err)

	must(expandEnviron(*decrypt))
	must(syscall.Exec(path, args[0:], os.Environ()))
}

type ssmVar struct {
	envvar    string
	parameter string
}

func expandEnviron(decrypt bool) error {
	// Environment variables that point to some SSM parameters.
	var ssmVars []ssmVar

	input := &ssm.GetParametersInput{
		WithDecryption: aws.Bool(decrypt),
	}

	for _, envvar := range os.Environ() {
		k, v := splitVar(envvar)
		if strings.HasPrefix(v, Prefix) {
			// The name of the SSM parameter.
			parameter := v[len(Prefix):]
			input.Names = append(input.Names, aws.String(parameter))
			ssmVars = append(ssmVars, ssmVar{k, parameter})
		}
	}

	if len(input.Names) == 0 {
		// Nothing to do, no SSM parameters.
		return nil
	}

	resp, err := client.GetParameters(input)
	if err != nil {
		return err
	}

	if len(resp.InvalidParameters) > 0 {
		var parameters []string
		for _, p := range resp.InvalidParameters {
			parameters = append(parameters, aws.StringValue(p))
		}
		return fmt.Errorf("invalid parameters: %v", parameters)
	}

	values := make(map[string]string)
	for _, p := range resp.Parameters {
		values[*p.Name] = *p.Value
	}

	for _, v := range ssmVars {
		os.Setenv(v.envvar, values[v.parameter])
	}

	return nil
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
