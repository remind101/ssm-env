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
$ git clone https://github.com/remind101/ssm-env && cd ssm-env
$ mv vendor/ src/ && GOPATH=$CWD make
$ echo "PATH=$CWD/bin:$PATH" >> ~/.bashrc
```
