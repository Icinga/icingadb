UPDATE icingadb_schema SET timestamp = UNIX_TIMESTAMP(timestamp / 1000) * 1000 WHERE timestamp > 20000000000000000;
