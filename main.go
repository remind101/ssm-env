package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/isobit/cli"
)

var Version string

const (
	// DefaultTemplate is the default template used to determine what the SSM
	// parameter name is for an environment variable.
	DefaultTemplate = `{{ if hasPrefix .Value "ssm://" }}{{ trimPrefix .Value "ssm://" }}{{ end }}`

	// GetParametersBatchSize is the default number of parameters to fetch at once.
	// The SSM API limits this to a maximum of 10 at the time of writing.
	GetParametersBatchSize = 10
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
	Version        bool     `cli:"short=V,help=print version"`
	Template       string
	WithDecryption bool     `cli:"short=d,help=attempt to decrypt SecureString parameters"`
	NoFail         bool     `cli:"help=ignore any errors getting parameters"`
	Args           []string `cli:"args,placeholder=<program> [args]"`
}

func (s *SSMEnv) Run() error {
	if s.Version {
		fmt.Println(Version)
		return nil
	}

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

	// Extract variables where executing the template returns a non-empty
	// string into the `ssmParams` map, which is a map of SSM param names to a
	// slice of env var keys.
	//
	// Variables for which the template returns an empty string are appended to
	// `env` as-is.
	ssmParams := map[string][]string{}
	for _, kv := range os.Environ() {
		key, val, _ := strings.Cut(kv, "=")

		b := strings.Builder{}
		data := struct{ Name, Value string }{key, val}
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
		// Lazily construct SSM client.
		if ssmClient == nil {
			cfg, err := awscfg.LoadDefaultConfig(context.TODO())
			if err != nil {
				return err
			}
			ssmClient = awsssm.NewFromConfig(cfg)
		}

		// Make the GetParameters API call.
		out, err := ssmClient.GetParameters(
			context.TODO(),
			&awsssm.GetParametersInput{
				Names:          names,
				WithDecryption: &s.WithDecryption,
			},
		)
		if err != nil {
			return err
		}
		for _, param := range out.InvalidParameters {
			return fmt.Errorf("invalid parameter: %s", param)
		}

		outMap := map[string]string{}
		for _, param := range out.Parameters {
			fullName := *param.Name
			if param.Selector != nil {
				fullName += *param.Selector
			}
			outMap[fullName] = *param.Value
		}
		for _, name := range names {
			val, ok := outMap[name]
			if !ok {
				return fmt.Errorf("GetParameters did not return a value for param: %s", name)
			}
			for _, key := range ssmParams[name] {
				env = append(env, key+"="+val)
			}
		}
		return nil
	}
	// Call GetParameters in batches.
	batch := []string{}
	i := 0
	for name := range ssmParams {
		batch = append(batch, name)
		// If this is the last batch, or the batch is full, get the batch.
		if i == len(ssmParams)-1 || len(batch) >= GetParametersBatchSize {
			if err := getParams(batch); err != nil {
				return nil, err
			}
			batch = []string{}
		}
		i += 1
	}

	return env, nil
}
