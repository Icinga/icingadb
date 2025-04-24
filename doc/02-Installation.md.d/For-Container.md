# Installing Icinga DB in Containers

This image integrates Icinga DB into your containerized environment.

## Usage

```bash
docker network create icinga

docker run --rm -d \
	--network icinga \
	--name redis-icingadb \
	redis

docker run -d \
	--network icinga \
	--name mariadb-icingadb \
	-e MYSQL_RANDOM_ROOT_PASSWORD=1 \
	-e MYSQL_DATABASE=icingadb \
	-e MYSQL_USER=icingadb \
	-e MYSQL_PASSWORD=123456 \
	mariadb

docker run -d \
	--network icinga \
	--restart always \
	--name icingadb \
	-e ICINGADB_REDIS_HOST=redis-icingadb \
	-e ICINGADB_REDIS_PORT=6379 \
	-e ICINGADB_REDIS_PASSWORD=123456 \
	-e ICINGADB_DATABASE_HOST=mariadb-icingadb \
	-e ICINGADB_DATABASE_PORT=3306 \
	-e ICINGADB_DATABASE_DATABASE=icingadb \
	-e ICINGADB_DATABASE_USER=icingadb \
	-e ICINGADB_DATABASE_PASSWORD=123456 \
	icinga/icingadb
```

The container doesn't need any volumes and takes the environment variables.

### Environment Variables

The environment variables consists of the key `ICINGADB_DATABASE_` and the configuration option name like `HOST` in uppercase.

```
ICINGADB_DATABASE_HOST=mariadb-icingadb
```

For more options, see the [Icinga DB configuration documentation].

### Connect via TLS

```bash
docker run -d \
	--network icinga \
	--restart always \
	--name icingadb \
	-e ICINGADB_REDIS_HOST=redis-icingadb \
	-e ICINGADB_REDIS_PORT=6379 \
	-e ICINGADB_REDIS_PASSWORD=123456 \
	-e ICINGADB_REDIS_TLS=true \
	-e ICINGADB_REDIS_CERT='[PEM-encoded content or path to a file (in a volume)]' \
	-e ICINGADB_REDIS_KEY='[PEM-encoded content or path to a file (in a volume)]' \
	-e ICINGADB_REDIS_CA='[PEM-encoded content or path to a file (in a volume)]' \
	-e ICINGADB_DATABASE_HOST=mariadb-icingadb \
	-e ICINGADB_DATABASE_PORT=3306 \
	-e ICINGADB_DATABASE_DATABASE=icingadb \
	-e ICINGADB_DATABASE_USER=icingadb \
	-e ICINGADB_DATABASE_PASSWORD=123456 \
	-e ICINGADB_DATABASE_TLS=true \
	-e ICINGADB_DATABASE_CERT='[PEM-encoded content or path to a file (in a volume)]' \
	-e ICINGADB_DATABASE_KEY='[PEM-encoded content or path to a file (in a volume)]' \
	-e ICINGADB_DATABASE_CA='[PEM-encoded content or path to a file (in a volume)]' \
	icinga/icingadb
```

## Build it yourself

```bash
git clone https://github.com/Icinga/icingadb.git

docker build -t icingadb --file Containerfile .
```

[Icinga DB configuration documentation]: https://icinga.com/docs/icingadb/latest/doc/03-Configuration/
