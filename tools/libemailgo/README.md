# A Drop in Replacement for Geneos `libemail.so`

`libemail.so` is intended as a drop-in replacement for standard `libemail.so` with the following extras:

* Enhanced, modern SMTP support
* TLS
* Authentication

Soon there will be one or more alterantives, the current aim is to provide:

* Go templates instead of existing format
* HTML / mobile friendly email

# Building

Either use `make` or

```go build -buildmode c-shared -o libemail.so main.go formats.go utils.go libemail.go```

# Using

The official  `libemail.so` is [documented here](https://docs.itrsgroup.com/docs/geneos/current/Gateway_Reference_Guide/geneos_rulesactionsalerts_tr.html#Libemail) and all of the parameters are supported.

Note that is you are using this as a literal drop-in replacement you will need to restart the Gateway process to load the new library.

The following additional parameters are supported by the `SendEMail` function:

* `_SMTP_USERNAME`
* `_SMTP_PASSWORD`
* `_SMTP_PASSWORD_FILE`
* `_SMTP_TLS`

If `_SMTP_USERNAME` is set then authentication is attempted. The password is either a *Plain Text* field, or to ensure this is not visible in the configuration it can be loaded from a file on the Gateway server. The `_SMTP_PASSWORD_FILE` is either and absolute path or a path to a file relative to the working directory of the Gateway.

`_SMTP_TLS` can be one of `default`, `force` or `none` (case insensitive). The default is to try TLS but fall back to plain text depending on the SMTP server.

If you use this library with authentication and connect to a public server, such as GMail or Outlook, then you should always create and use a unique "app password" and never use the real password for the sender account.
