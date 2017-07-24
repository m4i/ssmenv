# ssmenv

ssmenv is a tool to use Amazon EC2 Systems Manager (SSM) Parameter Store as environment variables.

```
$ aws ssm put-parameter --name /Foo/BAR --type String --value hello
$ aws ssm put-parameter --name /Foo/BAZ --type String --value world
$ ssmenv exec --path /Foo sh -c 'echo $BAR, $BAZ'
hello, world
```

[![CircleCI](https://circleci.com/gh/m4i/ssmenv.svg?style=shield)](https://circleci.com/gh/m4i/ssmenv)
[![Go Report Card](https://goreportcard.com/badge/github.com/m4i/ssmenv)](https://goreportcard.com/report/github.com/m4i/ssmenv)
[![codecov](https://codecov.io/gh/m4i/ssmenv/branch/master/graph/badge.svg)](https://codecov.io/gh/m4i/ssmenv)


## Install

Download a binary from [release page](https://github.com/m4i/ssmenv/releases) and place it in $PATH directory.


## Usage

```
ssmenv exec [--paths=PATH,PATH...] [--recursive] command ...
ssmenv get [--path=PATH] [--recursive] [--export] [name]
ssmenv set [--path=PATH] name=value ...
ssmenv replace --path=PATH [--recursive] name=value ...
```

## Example


Set parameters.  
`name=value` is a `String` type. `name@=value` is a `SecureString` type.

```
$ ssmenv set /Prod/DBNAME=prod /Prod/DBPASS@=passw0rd
PUT /Prod/DBNAME=prod
PUT /Prod/DBPASS@=****************
```

You can also set parameters from STDIN.

```
$ cat <<EOF > envfile
> /Staging/DBNAME=staging
> # comment lines begin with #
> /Staging/DBPASS@=pwd
> EOF

$ ssmenv set < envfile
PUT /Staging/DBNAME=staging
PUT /Staging/DBPASS@=****************
```

Set parameters with `--path` option.

```
$ ssmenv set --path /Common AWS_REGION=us-east-1 AWS_ACCESS_KEY_ID@=AKIAFOOBAR
PUT /Common/AWS_REGION=us-east-1
PUT /Common/AWS_ACCESS_KEY_ID@=****************
```

Get all parameters.

```
$ ssmenv get --recursive
/Common/AWS_ACCESS_KEY_ID@=AKIAFOOBAR
/Common/AWS_REGION=us-east-1
/Prod/DBNAME=prod
/Prod/DBPASS@=passw0rd
/Staging/DBNAME=staging
/Staging/DBPASS@=pwd
```

Execute the command with environment variables.

```
$ ssmenv exec --paths /Common,/Prod env
(snip)
AWS_ACCESS_KEY_ID=AKIAFOOBAR
AWS_REGION=us-east-1
DBNAME=prod
DBPASS=passw0rd

$ ssmenv exec --paths /Common,/Prod rails server
```

You can also export environment variables instead of executing the command directly.

```
$ ssmenv get --path /Prod --export
export DBNAME=prod
export DBPASS=passw0rd

$ $(ssmenv get --path /Common --export)
$ $(ssmenv get --path /Prod --export)
$ rails server
```

Replace all the parameters of the given path.

```
$ ssmenv replace --path /Prod DBNAME=prod DB_PASS@=passw0rd
UNCHANGED /Prod/DBNAME=prod
PUT /Prod/DB_PASS@=****************
DELETE /Prod/DBPASS
```

Other examples are in [cli_test.go](https://github.com/m4i/ssmenv/blob/master/cli_test.go).
