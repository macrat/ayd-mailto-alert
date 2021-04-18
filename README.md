Ayd? Mail Alert
===============

SMTP email alert sender for [Ayd?](https://github.com/macrat/ayd) status monitoring service.


## Usage

``` shell
$ export SMTP_SERVER=smtp.gmail.com:465
$ export SMTP_USERNAME=<< your username >>
$ export SMTP_PASSWORD=<< your password >>
$ export AYD_MAIL_TO="Jhon <jhon@example.com>, Alice <alice@example.com>"
$ export AYD_URL="http://ayd-external-url.example.com"

$ ayd -a exec:ayd-mail-alert ping:your-target.example.com
```


## Options

Set all options through environment variable.

|Variable       |Default                     |Description               |
|---------------|----------------------------|--------------------------|
|`SMTP_SERVER`  |                            |SMTP server name and port.|
|`SMTP_USERNAME`|                            |User name for SMTP server.|
|`SMTP_PASSWORD`|                            |Password for SMTP server. |
|`AYD_URL`      |`http://localhost:9000`     |Ayd? server address.      |
|`AYD_MAIL_FROM`|`Ayd? Alert <ayd@localhost>`|The From email address.   |
|`AYD_MAIL_TO`  |                            |The To email addresses.   |

Below options is set by Ayd? server.

|Variable        |Default|Description                                  |
|----------------|-------|---------------------------------------------|
|`ayd_target`    |       |The alerting target address.                 |
|`ayd_status`    |       |The status of target. "FAILURE" or "UNKNOWN".|
|`ayd_checked_at`|       |The timestamp of alert firing.               |
