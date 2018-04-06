# ssm-env

`ssm-env` is a simple UNIX tool to populate env vars from AWS Parameter Store.

## Usage

```console
ssm-env [-template STRING] [-with-decryption] COMMAND
```

## Details

Given the following environment:

```
RAILS_ENV=production
COOKIE_SECRET=ssm://prod.app.cookie-secret
```

You can run the application using `ssm-env` to automatically populate the `COOKIE_SECRET` env var from SSM:

```console
$ ssm-env env
RAILS_ENV=production
COOKIE_SECRET=super-secret
```

You can also configure how the parameter name is determined for an environment variable, by using the `-template` flag:

```console
$ export COOKIE_SECRET=xxx
$ ssm-env -template '{{ if eq .Name "COOKIE_SECRET" }}prod.app.cookie-secret{{end}}' env
RAILS_ENV=production
COOKIE_SECRET=super-secret
```

## Installation

```console
$ go get -u github.com/remind101/ssm-env
```

You can most likely find the downloaded binary in `~/go/bin/ssm-env`

### Usage with Docker

A common use case is to use `ssm-env` as a Docker ENTRYPOINT. You can copy and paste the following into the top of a Dockerfile:

```dockerfile
RUN curl -L https://github.com/remind101/ssm-env/releases/download/v0.0.2/ssm-env > /usr/local/bin/ssm-env && \
      cd /usr/local/bin && \
      echo ad0f184da3a6536d0614ce4133ceb23b  ssm-env | md5sum -c && \
      chmod +x ssm-env
ENTRYPOINT ["/usr/local/bin/ssm-env", "-with-decryption"]
```

Now, any command executed with the Docker image will be funneled through ssm-env.
