# Cassandra Driver

## Usage

```bash
migrate -url cassandra://host:port/keyspace -path ./db/migrations create add_field_to_table
migrate -url cassandra://host:port/keyspace -path ./db/migrations up
migrate help # for more info
```

Url format
- Authentication: `cassandra://username:password@host:port/keyspace`
- Cassandra v3.x: `cassandra://host:port/keyspace?protocol=4&consistency=all&disable_init_host_lookup`

> Cassandra in Docker users on a Mac: when using gcql + migrate, use the `disable_init_host_lookup` option in the connection URL. This will alleviate the issue of gocql trying to connect to internal docker IP addresses.

## Authors

* Paul Bergeron, https://github.com/dinedal
* Johnny Bergström, https://github.com/balboah
* pateld982, http://github.com/pateld982
