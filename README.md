Ayd? Mailto Alert
=================

SMTP email alert sender for [Ayd?](https://github.com/macrat/ayd) status monitoring service.


## Install

1. Download binary from [release page](https://github.com/macrat/ayd-mailto-alert/releases).

2. Save downloaded binary as `ayd-mailto-alert` to somewhere directory that registered to PATH.


## Usage

``` shell
$ export SMTP_SERVER=smtp.gmail.com:465
$ export SMTP_USERNAME=<< your username >>
$ export SMTP_PASSWORD=<< your password >>
$ export AYD_URL="http://ayd-external-url.example.com"

$ ayd -a mailto:your-email@example.com ping:your-target.example.com
```


## Options

Set all options through environment variable.

|Variable       |Default                     |Description               |
|---------------|----------------------------|--------------------------|
|`SMTP_SERVER`  |                            |SMTP server name and port.|
|`SMTP_USERNAME`|                            |User name for SMTP server.|
|`SMTP_PASSWORD`|                            |Password for SMTP server. |
|`AYD_URL`      |`http://localhost:9000`     |Ayd server address.       |
|`AYD_MAIL_FROM`|`Ayd? Alert <ayd@localhost>`|The From email address.   |
