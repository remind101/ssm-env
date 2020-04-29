package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	log "github.com/sirupsen/logrus"
)

const (
	// DefaultTemplate is the default template used to determine what the SSM
	// parameter name is for an environment variable.
	DefaultTemplate = `{{ if hasPrefix .Value "ssm://" }}{{ trimPrefix .Value "ssm://" }}{{ end }}`

	// defaultBatchSize is the default number of parameters to fetch at once.
	// The SSM API limits this to a maximum of 10 at the time of writing.
	defaultBatchSize = 10
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.WarnLevel)
}

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
		nofail  = flag.Bool("no-fail", false, "Don't fail if error retrieving parameter")
		debug = flag.Bool("debug", false, "Enable debug logs")
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
	if *debug {
		log.SetLevel(log.DebugLevel)
	}
	e := &expander{
		batchSize: defaultBatchSize,
		t:         t,
		ssm:       ssm.New(session.Must(awsSession(*debug))),
		os:        os,
	}
	must(e.expandEnviron(*decrypt, *nofail))
	must(syscall.Exec(path, args[0:], os.Environ()))
}

func awsSession(debug bool) (*session.Session, error) {
	config := aws.NewConfig()
	if debug {
		config.WithLogLevel(aws.LogDebugWithHTTPBody)
	}
	sess := session.Must(session.NewSession(config))
	if len(aws.StringValue(sess.Config.Region)) == 0 {
		log.Debug("aws session unable to detect the region, trying to check with ec2 metadata")
		meta := ec2metadata.New(sess)
		identity, err := meta.GetInstanceIdentityDocument()
		if err != nil {
			awsErr := err.(awserr.Error)
			if awsErr.Code() == "EC2MetadataRequestError" {
				return sess, nil
			}
			return nil, err
		}
		return session.NewSession(&aws.Config{
			Region: aws.String(identity.Region),
		})
	}
	return sess, nil
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
	t         *template.Template
	ssm       ssmClient
	os        environ
	batchSize int
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

func (e *expander) expandEnviron(decrypt bool, nofail bool) error {
	// Environment variables that point to some SSM parameters.
	var ssmVars []ssmVar

	uniqNames := make(map[string]bool)
	for _, envvar := range e.os.Environ() {
		k, v := splitVar(envvar)

		parameter, err := e.parameter(k, v)
		if err != nil {
			return fmt.Errorf("determining name of parameter: %v", err)
		}

		if parameter != nil {
			uniqNames[*parameter] = true
			ssmVars = append(ssmVars, ssmVar{k, *parameter})
		}
	}

	if len(uniqNames) == 0 {
		// Nothing to do, no SSM parameters.
		return nil
	}

	names := make([]string, len(uniqNames))
	i := 0
	for k := range uniqNames {
		names[i] = k
		i++
	}

	for i := 0; i < len(names); i += e.batchSize {
		j := i + e.batchSize
		if j > len(names) {
			j = len(names)
		}

		values, err := e.getParameters(names[i:j], decrypt, nofail)
		if err != nil {
			return err
		}

		for _, v := range ssmVars {
			val, ok := values[v.parameter]
			if ok {
				e.os.Setenv(v.envvar, val)
			}
		}
	}

	return nil
}

func (e *expander) getParameters(names []string, decrypt bool, nofail bool) (map[string]string, error) {
	values := make(map[string]string)

	input := &ssm.GetParametersInput{
		WithDecryption: aws.Bool(decrypt),
	}

	for _, n := range names {
		input.Names = append(input.Names, aws.String(n))
	}

	resp, err := e.ssm.GetParameters(input)
	if err != nil && ! nofail {
		return values, err
	}

	if len(resp.InvalidParameters) > 0 {
		if ! nofail {
			return values, newInvalidParametersError(resp)
		}
		fmt.Fprintf(os.Stderr, "ssm-env: %v\n", newInvalidParametersError(resp))
	}

	for _, p := range resp.Parameters {
		var name string
		if p.Selector != nil {
			name = *p.Name + *p.Selector
		} else {
			name = *p.Name
		}
		values[name] = *p.Value
	}

	return values, nil
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
