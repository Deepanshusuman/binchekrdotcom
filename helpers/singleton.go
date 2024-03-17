package helpers

import (
	"binchecker/credential"
	"context"
	"database/sql"
	"fmt"
	"sync"

	firebase "firebase.google.com/go"

	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/api/option"
)

var db *sql.DB
var client *redis.Client
var globalclient *redis.Client
var dblock = &sync.Mutex{}
var globalcachelock = &sync.Mutex{}
var cachelock = &sync.Mutex{}

func Db() *sql.DB {
	if db == nil {
		dblock.Lock()
		defer dblock.Unlock()
		if db == nil {
			db, _ = sql.Open(credential.SQL_DRIVER, credential.SQL_DATASOURCE)
			db.SetMaxIdleConns(0)
		}
	}
	return db
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
func GlobalRedis() *redis.Client {
	if globalclient == nil {
		globalcachelock.Lock()
		defer globalcachelock.Unlock()
		if globalclient == nil {
			globalclient = redis.NewClient(&redis.Options{Addr: credential.REDIS_GLOBAL_ADDRESS, Password: credential.REDIS_PASSWORD})
		}
	}
	return globalclient
}

var firebaseApp *firebase.App
var firebaseAppLock = &sync.Mutex{}

func FirebaseApp() *firebase.App {
	if firebaseApp == nil {
		firebaseAppLock.Lock()
		defer firebaseAppLock.Unlock()
		if firebaseApp == nil {
			opt := option.WithCredentialsFile("credential/firebase.json")
			app, err := firebase.NewApp(context.Background(), nil, opt)
			if err != nil {
				fmt.Println(err)
			}
			firebaseApp = app
		}
	}
	return firebaseApp
}
