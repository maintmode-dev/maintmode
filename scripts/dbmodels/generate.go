package main

import (
	"fmt"
	"log"
	"path"

	"github.com/go-jet/jet/v2/generator/metadata"
	mysqlgen "github.com/go-jet/jet/v2/generator/mysql"
	pggen "github.com/go-jet/jet/v2/generator/postgres"
	sqlitegen "github.com/go-jet/jet/v2/generator/sqlite"
	"github.com/go-jet/jet/v2/generator/template"
	"github.com/go-jet/jet/v2/mysql"
	"github.com/go-jet/jet/v2/postgres"
	"github.com/go-jet/jet/v2/sqlite"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	driverKey = "driver"
	dsnKey    = "dsn"
	schemaKey = "schema"
	destKey   = "dest"
)

type config struct {
	driver string
	dsn    string
	schema string
	dest   string
}

func main() {
	pflag.String(driverKey, "postgres", "Database driver: postgres, mysql, sqlite, etc")
	pflag.String(dsnKey, "", "DSN for database connection")
	pflag.String(schemaKey, "public", "Schema name for tables")
	pflag.String(destKey, "", "Destination dir for generated files")

	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		log.Fatalf("falied to bind flags: %v", err)
	}

	cfg := &config{
		driver: viper.GetString(driverKey),
		dsn:    viper.GetString(dsnKey),
		schema: viper.GetString(schemaKey),
		dest:   viper.GetString(destKey),
	}
	log.Printf("%s: %v", driverKey, cfg.driver)
	log.Printf("%s: %v", dsnKey, cfg.dsn)
	log.Printf("%s: %v", schemaKey, cfg.schema)
	log.Printf("%s: %v", destKey, cfg.dest)

	var err error
	switch cfg.driver {
	case "postgres":
		err = pgGenerator(cfg)
	case "mysql":
		err = mysqlGenerator(cfg)
	case "sqlite", "sqlite3":
		err = sqliteGenerator(cfg)
	default:
		err = fmt.Errorf("unsupported driver: %s", cfg.driver)
	}
	if err != nil {
		log.Fatalf("generate models failed: %v", err)
	}
}

func pgGenerator(cfg *config) error {
	return pggen.GenerateDSN(cfg.dsn, cfg.schema, cfg.dest,
		template.Default(postgres.Dialect).UseSchema(genTemplateSchema),
	)
}

func mysqlGenerator(cfg *config) error {
	cfg.dest = path.Join(cfg.dest, cfg.driver)
	return mysqlgen.GenerateDSN(cfg.dsn, cfg.dest,
		template.Default(mysql.Dialect).UseSchema(genTemplateSchema),
	)
}

func sqliteGenerator(cfg *config) error {
	cfg.dest = path.Join(cfg.dest, cfg.driver)
	return sqlitegen.GenerateDSN(cfg.dsn, cfg.dest,
		template.Default(sqlite.Dialect).UseSchema(genTemplateSchema),
	)
}

func genTemplateSchema(schemaMetaData metadata.Schema) template.Schema {
	return template.DefaultSchema(schemaMetaData).
		UseModel(template.DefaultModel().
			UseTable(func(table metadata.Table) template.TableModel {
				return template.DefaultTableModel(table).
					UseField(func(columnMetaData metadata.Column) template.TableModelField {
						defaultTableModelField := template.DefaultTableModelField(columnMetaData)

						//Добавляем тег db с именем столбца
						return defaultTableModelField.UseTags(
							fmt.Sprintf(`db:"%s.%s"`, table.Name, columnMetaData.Name),
						)
					})
			}),
		)
}
