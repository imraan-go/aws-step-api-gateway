package database

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/imraan-go/aws-step-order-service/config"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
	"github.com/uptrace/bun/extra/bunotel"
)

/*Create postgresql connection*/

func NewDB(conf *config.Database, debug bool) *bun.DB {
	var err error
	dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable&search_path=%s",
		conf.DbUser, conf.DbPass, conf.DbHost, conf.DbName, conf.DbSchema)
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	if err != nil {
		log.Fatal(err)
	}
	err = sqldb.Ping()
	if err != nil {
		log.Fatal(err)
	}
	sqldb.SetMaxOpenConns(conf.SetMaxOpenConns)
	db := bun.NewDB(sqldb, pgdialect.New(), bun.WithDiscardUnknownColumns())

	//if DEBUG=TRUE, enable sql query printing
	if debug {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	db.AddQueryHook(bunotel.NewQueryHook())
	return db
}
