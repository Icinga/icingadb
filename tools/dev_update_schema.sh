#!/bin/bash

set -e
set -o pipefail

cd "$(dirname "$0")"

BASEDIR=".."

test "$1" = '-f' || (
	echo "Run  '$0' -f  to DROP ALL ICINGADB TABLES and re-create them using the current schema"
	false
)

set -x

(
	mysql -uroot -e 'SHOW TABLES\G' icingadb </dev/null |\
	( grep -Ee '^Tables_in_icingadb: ' || true ) |\
	cut -d ' ' -f 2 |\
	perl -pe 's/^(.*?)$/DROP TABLE $1;/'

	mysql -uroot -e 'SHOW PROCEDURE STATUS\G' icingadb </dev/null |\
	( grep -Ee '^ *Name: ' || true ) |\
	cut -d ':' -f 2 |\
	perl -pe 's/^ *(.*?)$/DROP PROCEDURE IF EXISTS $1;/'

	cat $BASEDIR/etc/schema/mysql/{,helper/}*.sql

	echo "GRANT ALL ON icingadb.* TO 'module-dev'@'127.0.0.1' IDENTIFIED BY 'icinga0815';"
) |mysql -uroot icingadb
