`ssm-env` is a simple UNIX tool to populate env vars from AWS Parameter Store. Given the following environment:

```
RAILS_ENV=production
COOKIE_SECRET=ssm://prod.app.cookie-secret
```

You can run the application using `ssm-env` to automatically populate the `COOKIE_SECRET` env var from SSM:

```bash
$ ssm-env env
RAILS_ENV=production
COOKIE_SECRET=super-secret
```
