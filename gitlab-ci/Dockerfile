FROM golang
COPY icingadb-main /go/bin/icingadb-main
COPY etc/schema/mysql /icingadb-schema
COPY gitlab-ci/icingadb.ini /etc/icingadb.ini
CMD /go/bin/icingadb-main -config=/etc/icingadb.ini
