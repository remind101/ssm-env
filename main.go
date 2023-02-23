package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"

	"github.com/isobit/cli"
	// "github.com/aws/aws-sdk-go-v2"
	// awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	// DefaultTemplate is the default template used to determine what the SSM
	// parameter name is for an environment variable.
	DefaultTemplate = `{{ if hasPrefix .Value "ssm://" }}{{ trimPrefix .Value "ssm://" }}{{ end }}`

	// defaultBatchSize is the default number of parameters to fetch at once.
	// The SSM API limits this to a maximum of 10 at the time of writing.
	defaultBatchSize = 10
)

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

// func main() {
// 	var (
// 		template = flag.String("template", DefaultTemplate, "The template used to determine what the SSM parameter name is for an environment variable. When this template returns an empty string, the env variable is not an SSM parameter")
// 		decrypt  = flag.Bool("with-decryption", false, "Will attempt to decrypt the parameter, and set the env var as plaintext")
// 		nofail   = flag.Bool("no-fail", false, "Don't fail if error retrieving parameter")
// 	)
// 	flag.Parse()
// 	args := flag.Args()

// 	if len(args) <= 0 {
// 		flag.Usage()
// 		os.Exit(1)
// 	}

// 	path, err := exec.LookPath(args[0])
// 	must(err)

// 	var os osEnviron

// 	t, err := parseTemplate(*template)
// 	must(err)
// 	e := &expander{
// 		batchSize: defaultBatchSize,
// 		t:         t,
// 		ssm:       &lazySSMClient{},
// 		os:        os,
// 	}
// 	must(e.expandEnviron(*decrypt, *nofail))
// 	must(syscall.Exec(path, args[0:], os.Environ()))
// }

func main() {
	cmd := cli.New("ssm-env", &SSMEnv{
		Template: DefaultTemplate,
	})
	if err := cmd.Parse().Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ssm-env: %s\n", err)
		os.Exit(1)
	}
}

type SSMEnv struct {
	Template       string
	WithDecryption bool `cli:"short=d,help=attempt to decrypt SecureString parameters"`
	NoFail         bool `cli:"help=don't fail if there is an error getting a parameter"`
	Args           []string `cli:"args"`
}

func (s *SSMEnv) Run() error {
	if len(s.Args) <= 0 {
		return cli.UsageErrorf("at least one arg is required")
	}

	execName := s.Args[0]
	execPath, err := exec.LookPath(execName)
	if err != nil {
		return fmt.Errorf("could not find executable \"%s\": %w", execName, err)
	}

	tmpl, err := template.New("template").Funcs(TemplateFuncs).Parse(s.Template)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	execEnv, err := s.expandEnv(tmpl)
	if err != nil && !s.NoFail {
		return fmt.Errorf("error expanding env: %w", err)
	}

	if err := syscall.Exec(execPath, s.Args, execEnv); err != nil {
		return fmt.Errorf("exec error: %w", err)
	}

	return nil
}

func (s *SSMEnv) expandEnv(tmpl *template.Template) ([]string, error) {
	osEnv := os.Environ()
	env := make([]string, len(osEnv))
	ssmParams := map[string][]string{}
	for _, kv := range os.Environ() {
		key, val, _ := strings.Cut(kv, "=")

		b := strings.Builder{}
		data := struct { Name, Value string }{key, val}
		if err := tmpl.Execute(&b, data); err != nil {
			return nil, fmt.Errorf("template error: %w", err)
		}
		paramName := b.String()
		if paramName != "" {
			if keys, ok := ssmParams[paramName]; ok {
				ssmParams[paramName] = append(keys, key)
			} else {
				ssmParams[paramName] = []string{key}
			}
		} else {
			env = append(env, kv)
		}
	}

	var ssmClient *awsssm.Client
	getParams := func(names []string) error {
		fmt.Printf("get: %+v\n", names)
		if ssmClient == nil {
			cfg, err := awscfg.LoadDefaultConfig(context.TODO())
			if err != nil {
				return err
			}
			ssmClient = awsssm.NewFromConfig(cfg)
		}
		out, err := ssmClient.GetParameters(
			context.TODO(),
			&awsssm.GetParametersInput{
				Names: names,
				WithDecryption: &s.WithDecryption,
			},
		)
		if err != nil {
			return err
		}
		fmt.Printf("%+v\n", out)
		return nil
	}
	getParamBatches := func() error {
		batch := []string{}
		for name, _ := range ssmParams {
			batch = append(batch, name)
			if len(batch) < 10 {
				continue
			}
			if err := getParams(batch); err != nil {
				return err
			}
			batch = []string{}
		}
		if err := getParams(batch); err != nil {
			return err
		}
		return nil
	}
	if err := getParamBatches(); err != nil {
		return nil, err
	}

	for v, ks := range ssmParams {
		fmt.Printf("ssm var: %s; keys:", v)
		for k, _ := range ks {
			fmt.Printf(" %s", k)
		}
		fmt.Println()
	}

	return env, nil

	// awscfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	// if err != nil {
	// 	return err
	// }
}

// // lazySSMClient wraps the AWS SDK SSM client such that the AWS session and
// // SSM client are not actually initialized until GetParameters is called for
// // the first time.
// type lazySSMClient struct {
// 	ssm ssmClient
// }

// func (c *lazySSMClient) GetParameters(input *ssm.GetParametersInput) (*ssm.GetParametersOutput, error) {
// 	// Initialize the SSM client (and AWS session) if it hasn't been already.
// 	if c.ssm == nil {
// 		sess, err := c.awsSession()
// 		if err != nil {
// 			return nil, err
// 		}
// 		c.ssm = ssm.New(sess)
// 	}
// 	return c.ssm.GetParameters(input)
// }

// func (c *lazySSMClient) awsSession() (*session.Session, error) {
// 	sess, err := session.NewSession(&aws.Config{
// 		CredentialsChainVerboseErrors: aws.Bool(true),
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	// Clients will throw errors if a region isn't configured, so if one hasn't
// 	// been set already try to look up the region we're running in using the
// 	// EC2 Instance Metadata Endpoint.
// 	if len(aws.StringValue(sess.Config.Region)) == 0 {
// 		meta := ec2metadata.New(sess)
// 		identity, err := meta.GetInstanceIdentityDocument()
// 		if err == nil {
// 			sess.Config.Region = aws.String(identity.Region)
// 		}
// 		// Ignore any errors, the client will emit a missing region error
// 		// in the context of any parameter get calls anyway.
// 	}
// 	return sess, nil
// }

// func parseTemplate(templateText string) (*template.Template, error) {
// 	return template.New("template").Funcs(TemplateFuncs).Parse(templateText)
// }

// type ssmClient interface {
// 	GetParameters(*ssm.GetParametersInput) (*ssm.GetParametersOutput, error)
// }

// type environ interface {
// 	Environ() []string
// 	Setenv(key, vale string)
// }

// type osEnviron int

// func (e osEnviron) Environ() []string {
// 	return os.Environ()
// }

// func (e osEnviron) Setenv(key, val string) {
// 	os.Setenv(key, val)
// }

// type ssmVar struct {
// 	envvar    string
// 	parameter string
// }

// type expander struct {
// 	t         *template.Template
// 	ssm       ssmClient
// 	os        environ
// 	batchSize int
// }

// func (e *expander) parameter(k, v string) (*string, error) {
// 	b := new(bytes.Buffer)
// 	if err := e.t.Execute(b, struct{ Name, Value string }{k, v}); err != nil {
// 		return nil, err
// 	}

// 	if p := b.String(); p != "" {
// 		return &p, nil
// 	}

// 	return nil, nil
// }

// func (e *expander) expandEnviron(decrypt bool, nofail bool) error {
// 	// Environment variables that point to some SSM parameters.
// 	var ssmVars []ssmVar

// 	uniqNames := make(map[string]bool)
// 	for _, envvar := range e.os.Environ() {
// 		k, v := splitVar(envvar)

// 		parameter, err := e.parameter(k, v)
// 		if err != nil {
// 			// TODO: Should this _also_ not error if nofail is passed?
// 			return fmt.Errorf("determining name of parameter: %v", err)
// 		}

// 		if parameter != nil {
// 			uniqNames[*parameter] = true
// 			ssmVars = append(ssmVars, ssmVar{k, *parameter})
// 		}
// 	}

// 	if len(uniqNames) == 0 {
// 		// Nothing to do, no SSM parameters.
// 		return nil
// 	}

// 	names := make([]string, len(uniqNames))
// 	i := 0
// 	for k := range uniqNames {
// 		names[i] = k
// 		i++
// 	}

// 	for i := 0; i < len(names); i += e.batchSize {
// 		j := i + e.batchSize
// 		if j > len(names) {
// 			j = len(names)
// 		}

// 		values, err := e.getParameters(names[i:j], decrypt, nofail)
// 		if err != nil {
// 			return err
// 		}

// 		for _, v := range ssmVars {
// 			val, ok := values[v.parameter]
// 			if ok {
// 				e.os.Setenv(v.envvar, val)
// 			}
// 		}
// 	}

// 	return nil
// }

// func (e *expander) getParameters(names []string, decrypt bool, nofail bool) (map[string]string, error) {
// 	values := make(map[string]string)

// 	input := &ssm.GetParametersInput{
// 		WithDecryption: aws.Bool(decrypt),
// 	}

// 	for _, n := range names {
// 		input.Names = append(input.Names, aws.String(n))
// 	}

// 	resp, err := e.ssm.GetParameters(input)
// 	if err != nil && !nofail {
// 		return values, err
// 	}

// 	if len(resp.InvalidParameters) > 0 {
// 		if !nofail {
// 			return values, newInvalidParametersError(resp)
// 		}
// 		fmt.Fprintf(os.Stderr, "ssm-env: %v\n", newInvalidParametersError(resp))
// 	}

// 	for _, p := range resp.Parameters {
// 		var name string
// 		if p.Selector != nil {
// 			name = *p.Name + *p.Selector
// 		} else {
// 			name = *p.Name
// 		}
// 		values[name] = *p.Value
// 	}

// 	return values, nil
// }

// type invalidParametersError struct {
// 	InvalidParameters []string
// }

// func newInvalidParametersError(resp *ssm.GetParametersOutput) *invalidParametersError {
// 	e := new(invalidParametersError)
// 	for _, p := range resp.InvalidParameters {
// 		if p == nil {
// 			continue
// 		}

// 		e.InvalidParameters = append(e.InvalidParameters, *p)
// 	}
// 	return e
// }

// func (e *invalidParametersError) Error() string {
// 	return fmt.Sprintf("invalid parameters: %v", e.InvalidParameters)
// }

// func splitVar(v string) (key, val string) {
// 	parts := strings.Split(v, "=")
// 	return parts[0], parts[1]
// }

// func must(err error) {
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "ssm-env: %v\n", err)
// 		os.Exit(1)
// 	}
// }
