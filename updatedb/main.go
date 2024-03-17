package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

var ranges = [][]int64{
	{16777216, 167772159},
	{184549376, 1681915903},
	{1686110208, 2130706431},
	{2147483648, 2851995647},
	{2852061184, 2886729727},
	{2887778304, 3221225471},
	{3221225728, 3221225983},
	{3221226240, 3227017983},
	{3227018240, 3232235519},
	{3232301056, 3323068415},
	{3323199488, 3325256703},
	{3325256960, 3405803775},
	{3405804032, 3758096383},
}

// https://api.github.com/repos/sapics/ip-location-db/commits?path=geolite2-city/geolite2-city-ipv4-num.csv.gz
type Data struct {
	Commit struct {
		Author struct {
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

func main() {
	response, err := http.Get("https://api.github.com/repos/sapics/ip-location-db/commits?path=geolite2-city/geolite2-city-ipv4-num.csv.gz")
	if err != nil {
		fmt.Print(err.Error())
	}
	responseData, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	var responseObject []Data
	err = json.Unmarshal(responseData, &responseObject)
	if err != nil {
		fmt.Println(string(responseData))
		log.Fatal(err)
	}

	t, err := time.Parse(time.RFC3339, responseObject[0].Commit.Author.Date)
	if err != nil {
		fmt.Println(err)
	}

	if time.Since(t).Hours() < 12.5 {
		_, err := exec.Command("/bin/sh", "download.sh").Output()
		if err != nil {
			fmt.Printf("error %s", err)
		}
		add()
		compress()
		check()
		fmt.Println("Updated")
	} else {
		file, _ := os.OpenFile("ip.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
		file.Close()
		fmt.Println("No Update: Creating Empty File")

	}

}

func check() {
	data := GetcsvData("ip.csv")
	for i := range data {

		if i == 0 {
			continue
		}
		start := Atoi(data[i-1][1]) + 1
		end := Atoi(data[i][0])
		if start != end {

			inrange := false
			for _, a := range ranges {
				if start >= a[0] && start <= a[1] {
					inrange = true
				}
			}
			if inrange {
				fmt.Println("Not Found: ", start, IptoString(start), IptoString(end))
			}
		}
	}
}

func add() {
	start := time.Now()
	file, err := os.OpenFile("merged.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Println()
	}

	highprior := GetcsvData("geolite2-city-ipv4-num.csv")
	medprior := GetcsvData("dbip-city-ipv4-num.csv")
	for i, v := range highprior {

		if i == 0 {
			w := csv.NewWriter(file)
			w.Write([]string{v[0], v[1], v[2], v[3], v[5]})
			w.Flush()
			continue

		}

		start := Atoi(highprior[i-1][1]) + 1
		end := Atoi(highprior[i][0])
		if start != end {

			end = end - 1
			var startipdata []string
			index := FindIp(medprior, start)
			if index != -1 {
				startipdata = medprior[index]
			} else {
				inrange := false
				for _, a := range ranges {
					if start >= a[0] && start <= a[1] {
						inrange = true
					}
				}
				if inrange {
					fmt.Println("Not Found: ", start, IptoString(start), IptoString(end))
				}

				continue
			}

			for j := start; j <= end; j++ {
				medindex := FindIp(medprior, j)
				if medindex != -1 {
					if startipdata[2] != medprior[medindex][2] ||
						startipdata[3] != medprior[medindex][3] ||
						startipdata[5] != medprior[medindex][5] {
						w := csv.NewWriter(file)
						w.Write([]string{strconv.FormatInt(start, 10), strconv.FormatInt(j-1, 10), startipdata[2], startipdata[3], startipdata[5]})
						w.Flush()

						start = j
						startipdata = medprior[medindex]
					}

				}
			}
			w := csv.NewWriter(file)
			w.Write([]string{strconv.FormatInt(start, 10), strconv.FormatInt(end, 10), startipdata[2], startipdata[3], startipdata[5]})
			w.Write([]string{v[0], v[1], v[2], v[3], v[5]})
			w.Flush()

		} else {
			w := csv.NewWriter(file)
			w.Write([]string{v[0], v[1], v[2], v[3], v[5]})
			w.Flush()
		}

	}

	file.Close()
	fmt.Printf("Add took %v\n", time.Since(start))
}
func compress() {
	start := time.Now()
	file, err := os.OpenFile("ip.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Println()
	}

	data := GetcsvData("merged.csv")
	startipdata := data[0]
	for i := range data {

		if i == 0 {
			continue
		}
		if startipdata[2] != data[i][2] || startipdata[3] != data[i][3] || startipdata[4] != data[i][4] {
			w := csv.NewWriter(file)
			w.Write([]string{startipdata[0], data[i-1][1], data[i-1][2], data[i-1][3], data[i-1][4]})
			w.Flush()
			startipdata = data[i]
		}
	}
	file.Close()
	fmt.Println("compressing took: ", time.Since(start))

}

func binarySearch(a [][]string, search int64) (result int) {
	L := 0
	R := len(a) - 1
	for L <= R {
		mid := (L + R) / 2
		start, _ := strconv.ParseInt(a[mid][0], 10, 64)
		end, _ := strconv.ParseInt(a[mid][1], 10, 64)
		switch {
		case int64(start) < search:

			if search >= start && search <= end {
				return mid
			} else {
				L = mid + 1
			}

		case int64(start) > search:

			if search >= start && search <= end {
				return mid
			} else {
				R = mid - 1
			}

		default:
			return mid
		}
	}
	return -1
}

func IptoDecimal(ip string) int64 {
	IPv4Int := big.NewInt(0)
	IPv4Int.SetBytes(net.ParseIP(ip).To4())
	return IPv4Int.Int64()
}
func FindIp(list [][]string, ip int64) int {
	return binarySearch(list, ip)
}

// decimal ip to ip string
func IptoString(ip int64) string {
	return strconv.FormatInt(ip>>24, 10) + "." + strconv.FormatInt((ip>>16)&0xff, 10) + "." + strconv.FormatInt((ip>>8)&0xff, 10) + "." + strconv.FormatInt(ip&0xff, 10)
}

func Atoi(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func GetcsvData(filename string) [][]string {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	rawCSVdata, err := reader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	return rawCSVdata
}
