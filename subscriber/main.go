package main

import (
	"binchecker/credential"
	"database/sql"
	"fmt"
	"sync"

	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
)

var cache = Cache()
var db = Db()

var DB *sql.DB
var client *redis.Client
var dblock = &sync.Mutex{}
var cachelock = &sync.Mutex{}

func Db() *sql.DB {
	if DB == nil {
		dblock.Lock()
		defer dblock.Unlock()
		if DB == nil {
			DB, _ = sql.Open(credential.SQL_DRIVER, credential.SQL_DATASOURCE)
		}
	}
	return DB
}

func Cache() *redis.Client {
	if client == nil {
		cachelock.Lock()
		defer cachelock.Unlock()
		if client == nil {
			client = redis.NewClient(&redis.Options{Addr: credential.REDIS_LOCAL_ADDRESS, Password: credential.REDIS_PASSWORD})
		}
	}
	return client
}
func main() {
	subscriber := cache.Subscribe("query")
	for {
		msg, err := subscriber.ReceiveMessage()
		if err != nil {
			fmt.Println(err)
		}
		qr, err := db.Query(msg.Payload)
		if err != nil {
			fmt.Println("Query: ", msg.Payload)
			fmt.Println("Message: ", err.Error())
		} else {
			qr.Close()
		}

	}

}
