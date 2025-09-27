## rocketship login

Authenticate the CLI via OIDC device flow

### Synopsis

Launches the OAuth 2.0 device authorization flow for the active profile. The command displays a verification URL and code that must be approved in a browser. On success, the CLI stores short-lived access tokens (and refresh tokens when available) in the system keyring or an encrypted file fallback.

```
rocketship login [flags]
```

### Options

```
  -h, --help            help for login
  -p, --profile string  Profile to authenticate (defaults to active profile)
```

### Options inherited from parent commands

```
      --debug   Enable debug logging
```

### SEE ALSO

* [rocketship](rocketship.md)	 - Rocketship CLI

