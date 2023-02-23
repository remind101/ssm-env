package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"text/template"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/isobit/cli"
	"golang.org/x/text/cases"
)

var Version string

const (
	// defaultTemplate is the default template used to determine what the SSM
	// parameter name is for an environment variable.
	defaultTemplate = `{{ if hasPrefix .Value "ssm://" }}{{ trimPrefix .Value "ssm://" }}{{ end }}`

	// getParametersBatchSize is the default number of parameters to fetch at once.
	// The SSM API limits this to a maximum of 10 at the time of writing.
	getParametersBatchSize = 10
)

// templateFuncs are helper functions provided to the template.
var templateFuncs = template.FuncMap{
	"contains":   strings.Contains,
	"hasPrefix":  strings.HasPrefix,
	"hasSuffix":  strings.HasSuffix,
	"trimPrefix": strings.TrimPrefix,
	"trimSuffix": strings.TrimSuffix,
	"trimSpace":  strings.TrimSpace,
	"trimLeft":   strings.TrimLeft,
	"trimRight":  strings.TrimRight,
	"trim":       strings.Trim,
	"title":      cases.Title,
	"toTitle":    strings.ToTitle,
	"toLower":    strings.ToLower,
	"toUpper":    strings.ToUpper,
}

func main() {
	cmd := cli.New("ssm-env", &command{
		Template: defaultTemplate,
		Timeout:  10 * time.Second,
	})
	if err := cmd.Parse().Run(); err != nil {
		fmt.Fprintf(os.Stderr, "ssm-env: %s\n", err)
		os.Exit(1)
	}
}

type command struct {
	Version        bool `cli:"short=V,help=print version"`
	Debug          bool
	Template       string `cli:"short=t,help=template used to map env vars to SSM param names"`
	WithDecryption bool   `cli:"short=d,help=attempt to decrypt SecureString parameters"`
	NoFail         bool   `cli:"help=ignore any errors getting parameters"`
	Timeout        time.Duration
	Args           []string `cli:"args,placeholder=<program> [args]"`
}

func (s *command) Run() error {
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

	tmpl, err := template.New("template").Funcs(templateFuncs).Parse(s.Template)
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

func (cmd *command) expandEnv(tmpl *template.Template) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
	defer cancel()

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
			if cmd.Debug {
				fmt.Fprintf(os.Stderr, "ssm-env: found SSM env var: key=%s paramName=%s\n", paramName, key)
			}
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
			if cmd.Debug {
				fmt.Fprintf(os.Stderr, "ssm-env: loading AWS config\n")
			}
			cfg, err := awscfg.LoadDefaultConfig(ctx)
			if err != nil {
				return err
			}
			ssmClient = awsssm.NewFromConfig(cfg)
		}

		if cmd.Debug {
			fmt.Fprintf(os.Stderr, "ssm-env: GetParameters(%s)\n", strings.Join(names, ", "))
		}
		// Make the GetParameters API call.
		out, err := ssmClient.GetParameters(
			ctx,
			&awsssm.GetParametersInput{
				Names:          names,
				WithDecryption: &cmd.WithDecryption,
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
		if i == len(ssmParams)-1 || len(batch) >= getParametersBatchSize {
			if err := getParams(batch); err != nil {
				return nil, err
			}
			batch = []string{}
		}
		i += 1
	}

	return env, nil
}
