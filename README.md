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

### Usage with Alpine Node Docker Images

To use `ssm-env` with [Alpine Node](https://github.com/mhart/alpine-node) or other
[Alpine Linux](https://alpinelinux.org/) Docker images, root certificates need to be added and the installation
command differs, as shown in the `Dockerfile` below. Note that this example `Dockerfile` for Alpine Node also
includes [dumb-init](https://github.com/Yelp/dumb-init) which is recommended when running Node in containers.

```dockerfile
FROM node:11-alpine

# ...copy code, npm install etc.

# dumb-init: See https://github.com/Yelp/dumb-init
RUN wget -O /usr/local/bin/dumb-init https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64
RUN chmod +x /usr/local/bin/dumb-init

# ssm-env: See https://github.com/remind101/ssm-env
RUN wget -O /usr/local/bin/ssm-env https://github.com/remind101/ssm-env/releases/download/v0.0.2/ssm-env
RUN chmod +x /usr/local/bin/ssm-env

# Alpine Node doesn't include root certificates which ssm-env needs to talk to AWS.
# See https://simplydistributed.wordpress.com/2018/05/22/certificate-error-with-go-http-client-in-alpine-docker/
RUN apk add --no-cache ca-certificates

USER node
ENTRYPOINT ["/usr/local/bin/ssm-env", "-with-decryption", "/usr/local/bin/dumb-init", "--"]
CMD ["node", "server.js"]
EXPOSE 3000
```

## Usage with Kubernetes

A simple way to provide AWS credentials to `ssm-env` in containers run in Kubernetes is to use Kubernetes
[Secrets](https://kubernetes.io/docs/tasks/inject-data-application/distribute-credentials-secure/) and to expose
them as environment variables. There are more secure alternatives to environment variables, but if this is secure
enough for your needs, it provides a low-effort setup path.

First, store your AWS credentials in a secret called `aws-credentials`:

```shell
kubectl create secret generic aws-credentials --from-literal=AWS_ACCESS_KEY_ID='AKIA...' --from-literal=AWS_SECRET_ACCESS_KEY='...'
```

Then, in the container specification in your deployment or pod file, add them as environment variables (alongside
all other environment variables, including those retrieved from SSM):

```yaml
      containers:
        - env:
            - name: AWS_ACCESS_KEY_ID
              valueFrom:
                secretKeyRef:
                  name: aws-credentials
                  key: AWS_ACCESS_KEY_ID
            - name: AWS_SECRET_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: aws-credentials
                  key: AWS_SECRET_ACCESS_KEY
            - name: AWS_REGION
              value: us-east-1
            - name: SSM_EXAMPLE
              value: ssm:///foo/bar
```
