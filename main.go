package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// DefaultTemplate is the default template used to determine what the SSM
// parameter name is for an environment variable.
const DefaultTemplate = `{{ if hasPrefix .Value "ssm://" }}{{ trimPrefix .Value "ssm://" }}{{ end }}`

// TemplateFuncs are helper functions provided to the template.
var TemplateFuncs = template.FuncMap{
	"contains":   strings.Contains,
	"hasPrefix":  strings.HasPrefix,
	"hasSuffix":  strings.HasSuffix,
	"trimPrefix": strings.TrimPrefix,
	"trimSuffix": strings.TrimSuffix,
	"trimSpace":  strings.TrimSpace,
	"trimLeft":   strings.TrimLeft,
	"trimRight":  strings.TrimRight,
	"trim":       strings.Trim,
	"title":      strings.Title,
	"toTitle":    strings.ToTitle,
	"toLower":    strings.ToLower,
	"toUpper":    strings.ToUpper,
}

func main() {
	var (
		template = flag.String("template", DefaultTemplate, "The template used to determine what the SSM parameter name is for an environment variable. When this template returns an empty string, the env variable is not an SSM parameter")
		decrypt  = flag.Bool("with-decryption", false, "Will attempt to decrypt the parameter, and set the env var as plaintext")
	)
	flag.Parse()
	args := flag.Args()

	if len(args) <= 0 {
		flag.Usage()
		os.Exit(1)
	}

	path, err := exec.LookPath(args[0])
	must(err)

	var os osEnviron

	t, err := parseTemplate(*template)
	must(err)
	e := &expander{t: t, ssm: ssm.New(session.New()), os: os}
	must(e.expandEnviron(*decrypt))
	must(syscall.Exec(path, args[0:], os.Environ()))
}

func parseTemplate(templateText string) (*template.Template, error) {
	return template.New("template").Funcs(TemplateFuncs).Parse(templateText)
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
	t   *template.Template
	ssm ssmClient
	os  environ
}

func (e *expander) parameter(k, v string) (*string, error) {
	b := new(bytes.Buffer)
	if err := e.t.Execute(b, struct{ Name, Value string }{k, v}); err != nil {
		return nil, err
	}

	if p := b.String(); p != "" {
		return &p, nil
	}

	return nil, nil
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

		parameter, err := e.parameter(k, v)
		if err != nil {
			return fmt.Errorf("determining name of parameter: %v", err)
		}

		if parameter != nil {
			names[*parameter] = true
			ssmVars = append(ssmVars, ssmVar{k, *parameter})
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
