# A Drop in Replacement for Geneos `libemail.so`

`libemail.so` is intended as a drop-in replacement for standard `libemail.so` with the following extras:

* Enhanced, modern SMTP support
* TLS
* Authentication
* Go templates for both text and HTML
* HTML and CSS support
* Email meta parameters are removed from list available to formats

# Building

You must have Go 1.17 or later installed as well as `make`

```
git clone https://github.com/pgalbavy/geneos.git
cd geneos/tools/libemailgo
make
```

If you do not have `make` installed you can build using:

```
go build -buildmode c-shared -o libemail.so *.go
```

Then copy the resulting `libemail.so` to a suitable location and add the path to the Gateway configuration. You can replace the `libemail.so` file in the official distribution but you should probably backup the original file by renaming it:

```
mv libemail.so libemail.so.orig
```

# Using

## As a replacement for `libemail.so`

The official `libemail.so` is [documented here](https://docs.itrsgroup.com/docs/geneos/current/Gateway_Reference_Guide/geneos_rulesactionsalerts_tr.html#Libemail) and all of the parameters are supported.

Note that if you are using this as a literal drop-in replacement you will need to restart the Gateway process to load the new library.

### New features

The following additional parameters are supported by the `SendEMail` function:

* `_SMTP_USERNAME`
* `_SMTP_PASSWORD`
* `_SMTP_PASSWORD_FILE`
* `_SMTP_TLS`

If `_SMTP_USERNAME` is set then authentication is attempted. The password is either a *Plain Text* field, or to ensure this is not visible in the configuration it can be loaded from a file on the Gateway server. The `_SMTP_PASSWORD_FILE` is either and absolute path or a path to a file relative to the working directory of the Gateway. If both `_SMTP_PASSWORD` and `_SMTP_PASSWORD_FILE` are defined, `_SMTP_PASSWORD_FILE` is preferred.

`_SMTP_TLS` can be one of `default`, `force` or `none` (case insensitive). The default is to try TLS but fall back to plain text depending on the SMTP server.

If you use this library with authentication and connect to a public server, such as GMail or Outlook, then you should always create and use a unique "app password" and never use the real password for the sender account.

## `GoSendMail` function

This is a forward compatible function that accepts almost the standard parameters (except those with `FORMAT` in the name) above but will also add an HTML part to the EMail.

### Go templates

The following new parameters are used to support Go templates - both text and HTML:

* `_TEMPLATE_TEXT`
  Override the built-in text template. This is now a single block of configurable text and uses the power of Go templates to embed the logic to evaluate different Alert types that was previously performed in code and with multiple formats.
* `_TEMPLATE_TEXT_FILE`
  Override the build-in text template with the contents of the named file. This takes precendece over `_TEMPLATE_TEXT`
* `_TEMPLATE_TEXT_ONLY`
  If this is set (to any value) then the function will send a text-only EMail and not process any HTML or CSS settings
* `_TEMPLATE_HTML` and `_TEMPLATE_HTML_FILE`
  Similar to the above, these settings override the default HTML template. The default HTML template is almost identical to the text one except all parameter values are rendered in *bold*.
* `_TEMPLATE_CSS` and `_TEMPLATE_CSS_FILE`
  Similar to the above, these settings override the default CSS template. The CSS template is included in the HTML template, whether default or user defined, using the following syntax and should be enclosed in `<style type="text/css>...</style>` tags:
  > ```{{template "css"}}```
* `_TEMPLATE_LOGO_FILE`
  Override the default embedded logo, which is a Material notification icon. This should be a PNG file and is referenced in the HTML as
  > ```<img src="cid:logo.png" />```

  In a future version it is expected that multiple files will be loadable using Go's embed FS features.

## Debug

There is one built in `_DEBUG` parameter but you can also add your own to the template logic and `_TEMPLATE_DEBUG` has been included in the built-in templates to demonstrate this.

* `_DEBUG`
  If set to `true` (case insentisive) prevents EMail meta parameters (e.g. `_FROM`, `_SMTP_SERVER` etc.) from being removed from the parameters passed to formats or templates. This includes the plain text password, if provided, so beware. You can then output these values in your custom formats and templates for review.
* `_TEMPLATE_DEBUG`
  This example parameter outputs a text and HTML table of all parameters, unsorted, which may or may not include the EMail meta-parameters, depending on `_DEBUG` above. In the built-in templates this has to be either `TRUE` or `true` and will not work for `True`, for example.

