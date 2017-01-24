# Sqlite3 Driver

* Runs migrations in transactions.
  That means that if a migration fails, it will be safely rolled back.
* Tries to return helpful error messages.
* Stores migration version details in table ``schema_migrations``.
  This table will be auto-generated.


## Usage

```bash
migrate -url sqlite3://database.sqlite -path ./db/migrations create add_field_to_table
migrate -url sqlite3://database.sqlite -path ./db/migrations up
migrate help # for more info
```

## Authors

* Matthias Kadenbach, https://github.com/gemnasium
* Caesar Wirth, https://github.com/cjwirth
