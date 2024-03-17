package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
)

var cache = GlobalRedis()

func main() {
	// r := GetcsvData("visa.csv")
	// var startValues []string
	// for _, v := range r {
	// 	// delete v[0]
	// 	startValues = append(startValues, fmt.Sprintf("%s", v[0]))
	// }
	// query := fmt.Sprintf("DELETE FROM bins WHERE start IN (%s)", strings.Join(startValues, ","))
	// println(query)
	// Cleanvisa()
	addtobins()

}
func addtobins() {
	t := time.Now().UnixNano() / int64(time.Millisecond)
	r := GetcsvData("visa.csv")
	for _, v := range r {
		//q := "CALL add_bin(" + v[0] + "," + v[1] + ",'m','" + v[2] + "','" + v[3] + "','" + strings.ReplaceAll(v[4], "'", "''") + "','" + strings.ReplaceAll(v[6], "'", "''") + "','" + v[5] + "', " + strconv.FormatInt(t, 10) + ")"
		qr := "INSERT INTO bins (start,end,flag,network,type,product_name,issuer,country,updated_at) VALUES (" + v[0] + "," + v[1] + ",'v','" + v[2] + "','" + v[3] + "','" + strings.ReplaceAll(v[4], "'", "''") + "','" + strings.ReplaceAll(v[6], "'", "''") + "','" + v[5] + "', " + strconv.FormatInt(t, 10) + ")   ON DUPLICATE KEY UPDATE info = CONCAT(info , 'Duplicate - Start: " + v[0] + " End: " + v[1] + " Network: " + v[2] + " Type: " + v[3] + " Product: " + strings.ReplaceAll(v[4], "'", "''") + " Issuer: " + strings.ReplaceAll(v[6], "'", "''") + " Country: " + v[5] + "')"
		if err := cache.Publish("query", qr).Err(); err != nil {
			panic(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// create add_bin procedure if the row exists update it

}
func Cleanvisa() {
	file, err := os.OpenFile("visa.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Println()
	}

	r := GetcsvData("latest.csv")

	for _, v := range r {
		if v[0][6:] == "000" && v[1][6:] == "999" {
			w := csv.NewWriter(file)
			w.Write([]string{v[0] + "00", v[1] + "99", "Visa", strings.Title(strings.ToLower(v[4])), v[3], "IN", v[2]})
			w.Flush()
		}

	}
	file.Close()

}

// helper functions
var globalcachelock = &sync.Mutex{}
var globalclient *redis.Client

func GlobalRedis() *redis.Client {
	if globalclient == nil {
		globalcachelock.Lock()
		defer globalcachelock.Unlock()
		if globalclient == nil {
			globalclient = redis.NewClient(&redis.Options{Addr: "sg.binchekr.com:6379", Password: "Ix3N!hbR&NMDzjAfaXjmG!qzUnR4%$MZyjd?j#VdGuC$XAh5ktHxABDRA?HAdXx9achaGTUB$NdMDv$!Qu5J$zCVJ7S6PEnDqqdIT$bsGKBBZD$e%h5*dAqr#k8XCDp$qAURq%QHZptPV6CIKrqTXzsVgBuv88gt3gGISSyp2rE6UY$vH9we%Y@h@4kA^Zgr%3#IgWYxv2cPh4@I$Dv59$Qn$g3wb9*yp59ZR#D5N*WHI&F6Cz*sC3^A??nv#*DP"})
		}
	}
	return globalclient
}
func GetcsvData(filePath string) [][]string {
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0777)
	if err != nil {
		log.Fatal("Unable to read input file "+filePath, err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal("Unable to parse file as CSV for "+filePath, err.Error())
	}
	return records
}
