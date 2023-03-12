package db

import (
	"database/sql"
	"log"
	"os"
	"testing"

	"github.com/gin/simplebank/util"
	_ "github.com/lib/pq"
)

var testqueries *Queries
var testDB *sql.DB

func TestMain(m *testing.M) {
	config, err := util.LoadConfig("../../")
	if err != nil {
		log.Fatalf("can not read config: %s", err)
	}
	testDB, err = sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		log.Fatal("can not connect: ", err)
	}
	testqueries = New(testDB)

	// 今回は、accounts のみにする
	os.Exit(m.Run())
}
