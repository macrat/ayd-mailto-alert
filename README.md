ayd-mailto-alert
================

SMTP email alert sender for [Ayd](https://github.com/macrat/ayd) alive monitoring tool.


## Install

1. Download binary from [release page](https://github.com/macrat/ayd-mailto-alert/releases).

2. Save downloaded binary as `ayd-mailto-alert` to somewhere directory that registered to PATH.


## Usage

### Use mailrc

``` shell
$ cat ~/.mailrc
set smtp=smtps://smtp.gmail.com
set smtp-auth-user="your username"
set smtp-auth-password="your password"
set from="your name <your-email@example.com>"

$ export AYD_URL="http://ayd-external-url.example.com"

$ ayd -a mailto:your-email@example.com ping:your-target.example.com
```

### Use environment variable

``` shell
$ export SMTP_SERVER=smtp.gmail.com:465
$ export SMTP_USERNAME=$YOUR_USERNAME
$ export SMTP_PASSWORD=$YOUR_PASSWORD
$ export AYD_URL="http://ayd-external-url.example.com"

$ ayd -a mailto:your-email@example.com ping:your-target.example.com
```


## Options

Set all options through environment variable.

| Variable        | Default                     | Description                        |
|-----------------|-----------------------------|------------------------------------|
| `SMTP_SERVER`   |                             | SMTP server name and port.         |
| `SMTP_USERNAME` |                             | User name for SMTP server.         |
| `SMTP_PASSWORD` |                             | Password for SMTP server.          |
| `AYD_URL`       |                             | Ayd server address. (optional)     |
| `AYD_MAIL_FROM` | `Ayd Alert <ayd@localhost>` | The From email address. (optional) |

`AYD_MAIL_FROM` can also be set by `from` query in URL, for example, `mailto:your-email@example.com?from=your-server@example.com`.
