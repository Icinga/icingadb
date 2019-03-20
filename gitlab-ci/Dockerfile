FROM golang
COPY icingadb-main /go/bin/icingadb-main
COPY etc/schema/mysql /icingadb-schema
COPY icingadb.ini /icingadb.ini
CMD /go/bin/icingadb-main -config=/icingadb.ini
