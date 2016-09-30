# Apache Phoenix Driver

* Runs migrations in transactions.
  That means that if a migration failes, it will be safely rolled back.
* Tries to return helpful error messages.
* Stores migration version details in table ``schema_migrations``.
  This table will be auto-generated.


## Usage

```bash
migrate -url phoenix://host:port/schema -path ./db/migrations create add_field_to_table
migrate -url phoenix://host:port/schema -path ./db/migrations up
migrate help # for more info
```

See full [DSN (Data Source Name) documentation](https://github.com/Boostport/avatica#dsn-data-source-name).

## Authors

* Francis Chuang, https://github.com/F21, https://github.com/Boostport