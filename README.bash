#!/bin/bash

run() {
  echo "$ $@" | sed '2,$ s/^/> /'
  eval "$@"
  echo
}

run ssmenv set /Prod/DBNAME=prod /Prod/DBPASS@=passw0rd

run 'cat <<EOF > envfile
/Staging/DBNAME=staging
# comment lines begin with #
/Staging/DBPASS@=pwd
EOF'

run 'ssmenv set < envfile'

run ssmenv set --path /Common AWS_REGION=us-east-1 AWS_ACCESS_KEY_ID@=AKIAFOOBAR

run ssmenv get --recursive

run ssmenv exec --paths /Common,/Prod env

run ssmenv get --path /Prod --export

run ssmenv replace --path /Prod DBNAME=prod DB_PASS@=passw0rd
