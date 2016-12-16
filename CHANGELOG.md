# Migrate Changelog

## v1.4.1 - 2016-12-16

* [cassandra] Add [disable_init_host_lookup](https://github.com/gocql/gocql/blob/master/cluster.go#L92) url param (@GeorgeMac / #17)

## v1.4.0 - 2016-11-22

* [crate] Add [Crate](https://crate.io) database support, based on the Crate sql driver by [herenow](https://github.com/herenow/go-crate) (@dereulenspiegel / #16)

## v1.3.2 - 2016-11-11

* [sqlite] Allow multiple statements per migration (dklimkin / #11)

## v1.3.1 - 2016-08-16

* Make MySQL driver aware of SSL certificates for TLS connection by scanning ENV variables (https://github.com/mattes/migrate/pull/117/files)

## v1.3.0 - 2016-08-15

* Initial changelog release
* Timestamp migration, instead of increments (https://github.com/mattes/migrate/issues/102)
* Versions will now be tagged
* Added consistency parameter to cassandra connection string (https://github.com/mattes/migrate/pull/114)
