## rocketship logout

Remove stored authentication tokens

### Synopsis

Deletes the cached access and refresh tokens for the selected profile. Subsequent commands will require a new `rocketship login` before they can reach OIDC-protected engines.

```
rocketship logout [flags]
```

### Options

```
  -h, --help            help for logout
  -p, --profile string  Profile to clear (defaults to active profile)
```

### Options inherited from parent commands

```
      --debug   Enable debug logging
```

### SEE ALSO

* [rocketship](rocketship.md)	 - Rocketship CLI

