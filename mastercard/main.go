package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
)

var cache = GlobalRedis()

func main() {
	//addtobins()
	Sortmastercard()
}

func addtobins() {
	timer()()
	//Sortmastercard()
	t := time.Now().UnixNano() / int64(time.Millisecond)
	r := GetcsvData("mastercard.csv")
	for _, v := range r {
		//q := "CALL add_bin(" + v[0] + "," + v[1] + ",'m','" + v[2] + "','" + v[3] + "','" + strings.ReplaceAll(v[4], "'", "''") + "','" + strings.ReplaceAll(v[6], "'", "''") + "','" + v[5] + "', " + strconv.FormatInt(t, 10) + ")"
		qr := "INSERT INTO bins (start,end,flag,network,type,product_name,issuer,country,updated_at) VALUES (" + v[0] + "," + v[1] + ",'m','" + v[2] + "','" + v[3] + "','" + strings.ReplaceAll(v[4], "'", "''") + "','" + strings.ReplaceAll(v[6], "'", "''") + "','" + v[5] + "', " + strconv.FormatInt(t, 10) + ")   ON DUPLICATE KEY UPDATE info = CONCAT(info , 'Duplicate - Start: " + v[0] + " End: " + v[1] + " Network: " + v[2] + " Type: " + v[3] + " Product: " + strings.ReplaceAll(v[4], "'", "''") + " Issuer: " + strings.ReplaceAll(v[6], "'", "''") + " Country: " + v[5] + "')"
		if err := cache.Publish("query", qr).Err(); err != nil {
			panic(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// create add_bin procedure if the row exists update it

}

func Sortmastercard() {
	defer timer()()
	file, err := os.OpenFile("mastercard.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0777)
	if err != nil {
		fmt.Println()
	}

	r := GetcsvData("latest.csv")
	sort.Sort(CSV(r))

	for _, v := range r {
		c := v[7]
		if c == "ROM" {
			c = "ROU"
		}
		if c == "TMP" {
			c = "TLS"
		}
		if c == "QZZ" {
			c = "KOS"
		}
		if c == "ZAR" {
			c = "COD"
		}
		//fmt.Println(v)

		//EURO KARTENSYSTEME GMBH,11625,5303561,,,,,
		w := csv.NewWriter(file)
		w.Write([]string{v[2] + "0", v[3] + "9", GetCardBrand(v[6]), GetCardType(v[6]), GetCleanLevel(v[5]), co[c].Alpha2, v[0]})
		w.Flush()

	}
	file.Close()

}

func timer() func() {
	start := time.Now()
	return func() {
		fmt.Printf("took %v\n", time.Since(start))
	}
}

type Country struct {
	Alpha2 string `json:"alpha2"`
	Alpha3 string `json:"alpha3"`
}

var co = Byalpha3()

func Byalpha3() map[string]*Country {
	file, err := os.Open("../helpers/countries.json")

	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()
	var countries map[string]*Country
	// var alpha3 map[string]*Country
	alpha3 := make(map[string]*Country)
	decoder := json.NewDecoder(file)
	decoder.Decode(&countries)

	for _, v := range countries {
		alpha3[v.Alpha3] = v
	}
	return alpha3
}

func Emoji(iso2 string) string {
	buf := [...]byte{240, 159, 135, 0, 240, 159, 135, 0}
	buf[3] = iso2[0] + (166 - 'A')
	buf[7] = iso2[1] + (166 - 'A')
	return string(buf[:])
}

type CSV [][]string

// Determine if one CSV line at index i comes before the line at index j.
func (data CSV) Less(i, j int) bool {
	data1, _ := strconv.ParseInt(data[i][2], 10, 64)
	data2, _ := strconv.ParseInt(data[j][2], 10, 64)

	return data1 < data2

}

// Other functions required for sort.Sort.
func (data CSV) Len() int {
	return len(data)
}
func (data CSV) Swap(i, j int) {
	data[i], data[j] = data[j], data[i]
}

func GetCleanLevel(s string) string {
	b := strings.Split(s, "-")
	if len(b) > 1 {
		return strings.TrimLeft(b[1], " ")
	} else {
		return s
	}
	// 	CIR	CIRRUS
	// MSI	MAESTRO
	// MPG	MASTERCARD PREPAID GENERAL SPEND
	// MRW	MASTERCARD PREPAID BUSINESS NON US
	// MPL	PLATINUM
	// MCS	STANDARD
	// MCG	GOLD
	// MRG	MASTERCARD PREPAID NON US GENERAL SPEND
	// MDS	DEBIT STANDARD
	// MET	TITANIUM DEBIT MASTERCARD
	// MDU	DEBIT MASTERCARD UNEMBOSSED
	// MCT	TITANIUM MASTERCARD
	// MCW	WORLD
	// MDP	DEBIT PLATINUM
	// MDH	WORLD DEBIT EMBOSSED
	// MDT	COMMERCIAL DEBIT
	// MCP	PURCHASING CARD
	// MUS	PREPAID MASTERCARD UNEMBOSSED
	// OLR	PREPAID MSI CONSUMER RELOAD
	// MDG	DEBIT GOLD
	// MCO	MASTERCARD CORPORATE
	// MBS	MASTERCARD B2B PRODUCT 1
	// MGS	PLATINUM MASTERCARD PREPAID GENERAL SPEND
	// MWE	MASTERCARD WORLD ELITE
	// MDJ	DEBIT MASTERCARD ENHANCED
	// MDB	DEBIT BUSINESSCARD
	// MBK	MASTERCARD BLACK
	// MLC	MASTERCARD MICRO BUSINESS CARD
	// MCC	MASTERCARD MIXED
	// SUR	PREPAID MASTERCARD UNEMBOSSED NON US
	// PVL	PRIVATE LABEL
	// MBA	MASTERCARD B2B PRODUCT 2
	// MBG	MASTERCARD B2B PRODUCT 3
	// MBH	MASTERCARD B2B PRODUCT 4
	// MBI	MASTERCARD B2B PRODUCT 5
	// MBJ	MASTERCARD B2B PRODUCT 6
	// SPP	MASTERCARD INSTALLMENT PAYMENTS P
	// MAB	WORLD ELITE MASTERCARD BUSINESS
	// MCB	BUSINESSCARD
	// MPF	MASTERCARD PREPAID GIFT
	// MIU	DEBIT MASTERCARD UNEMBOSSED NON US
	// MCU	MASTERCARD UNEMBOSSED
	// MWB	WORLD MASTERCARD FOR BUSINESS
	// MLA	MASTERCARD CENTRAL TRAVEL SOLUTION AIR
	// MDW	WORLD ELITE DEBIT MASTERCARD
	// BPD	WORLD DEBIT BUSINESSCARD
	// MXG	DIGITAL ENABLEMENT PROGRAM
	// ACS	DIGITAL DEBIT
	// OLS	MAESTRO DELAYED DEBIT
	// MSB	MAESTRO SMALL BUSINESS
	// MNW	MASTERCARD NEW WORLD
	// MHH	HSA NON SUBSTANTIATED
	// MBP	MASTERCARD CORP PREPAID
	// TCS	MASTERCARD STANDARD CARD IMMEDIATE DEBIT
	// TPL	PLATINUM MASTERCARD IMMEDIATE DEBIT
	// TNW	MASTERCARD NEW WORLD IMMEDIATE DEBIT
	// MPW	MASTERCARD PREPAID WORKPLACE B2B
	// MPB	MASTERCARD PREFERRED BUSINESSCARD
	// MPX	MASTERCARD PREPAID FLEX BENEFIT
	// MRC	MCELECTRONIC PREPAID CONSUMER NON US
	// MEB	EXECUTIVE
	// MCE	MASTERCARD ELECTRONIC
	// MEO	CORPORATE EXECUTIVE
	// MPO	MASTERCARD PREPAID OTHER
	// MPY	MASTERCARD PREPAID EMPLOYEE INCENTIVE
	// MPA	MASTERCARD PREPAID PAYROLL
	// MPV	MASTERCARD PREPAID GOVT
	// MRH	MASTERCARD PREPAID PLATINUM TRAVEL
	// MPR	MASTERCARD PREPAID TRAVEL
	// MPM	MASTERCARD PREPAID CONSUMER INCENTIVE
	// MPN	MASTERCARD PREPAID INSURANCE
	// MBD	MC PROFESSIONAL DEBIT BUSINESSCARD
	// MIP	MASTERCARD PREPAID STUDENT CARD
	// MRL	MASTERCARD PREPAID BUSINESS PREFERRED
	// MDO	DEBIT OTHER
	// WPD	WORLD PREPAID TRAVEL DEBIT
	// MAC	MASTERCARD CORPORATE WORLD ELITE
	// TCO	MASTERCARD CORPORATE IMMEDIATE DEBIT
	// WBE	MASTERCARD WORLD BLACK EDITION
	// MKH	NEBULA CONSUMER CREDIT ULTRA HIGH NET WORTH
	// MBE	MASTERCARD ELECTRONIC BUSINESS
	// SPS	MASTERCARD INSTALLMENT PAYMENTS S
	// ETA	MASTERCARD INSTALLMENT PAYMENTS A
	// ETB	MASTERCARD INSTALLMENT PAYMENTS B
	// ETC	MASTERCARD INSTALLMENT PAYMENTS C
	// ETD	MASTERCARD INSTALLMENT PAYMENTS D
	// ETE	MASTERCARD INSTALLMENT PAYMENTS E
	// ETF	MASTERCARD INSTALLMENT PAYMENTS F
	// ETG	MASTERCARD INSTALLMENT PAYMENTS G
	// MRJ	PREPAID MASTERCARD VOUCHER MEAL FOOD CARD
	// MRF	EUROPEAN REGULATED INDIVIDUAL PAY
	// MWO	MASTERCARD CORPORATE WORLD
	// MAQ	MASTERCARD PREPAID COMMERCIAL PAYMENTS ACCOUNT
	// TCG	GOLD MASTERCARD IMMEDIATE DEBIT
	// MPC	MASTERCARD PROFESSIONAL CARD
	// TIU	MASTERCARD UNEMBOSSED IMMEDIATE DEBIT
	// TCW	WORLD ELITE MASTERCARD IMMEDIATE DEBIT
	// MRK	PREPAID MASTERCARD PUBLIC SECTOR COMMERCIAL
	// MLD	MASTERCARD DISTRIBUTION CARD
	// TCB	MASTERCARD BUSINESS CARD IMMEDIATE DEBIT
	// TIC	MASTERCARD STUDENT CARD IMMEDIATE DEBIT
	// MHP	HELOC PLATINUM MASTERCARD
	// BPC	BILL PAY FOR COMMERCIAL
	// MVG	MASTERCARD B2B VIP 7
	// MVN	MASTERCARD B2B VIP 14
	// MVX	MASTERCARD B2B VIP 24
	// FIA	MASTERCARD B2B VIP 27
	// FID	MASTERCARD B2B VIP 30
	// SAP	PLATINUM MASTERCARD SALARY IMMEDIATE DEBIT
	// GCP	MASTERCARD SHOP SPLIT PREMIUM
	// MHA	HEALTHCARE PREPAID NON TAX
	// WMR	WORLD REWARDS EDITION
	// SAS	STANDARD MASTERCARD SALARY IMMEDIATE DEBIT
	// WDR	WORLD DEBIT MASTERCARD REWARDS
	// MRD	PLATINUM DEBIT MASTERCARD PREPAID GENERAL SPEND
	// MIS	DEBIT MASTERCARD STUDENT CARD
	// MBW	WORLD BLACK EDITION DEBIT
	// MAP	MASTERCARD COMMERCIAL PAYMENTS ACCOUNT
	// MTP	MASTERCARD PREPAID PREMIUM TRAVEL
	// MLB	MASTERCARD BRAZIL BENEFIT FOR HOME IMPROVEMENT
	// MHB	HSA SUBSTANTIATED
	// MHS	HELOC STANDARD MASTERCARD
	// MCF	FLEET
	// MGF	MASTERCARD CORPORATE FLEET CARD GSA PROGRAM
	// MNF	MASTERCARD CORPORATE FLEET CARD MULTICARD PROGRAM FOR NON GSA PUBLIC SECTOR
	// MIC	MASTERCARD STUDENT CARD
	// MUP	PLATINUM DEBIT MASTERCARD UNEMBOSSED
	// MWF	MASTERCARD HUMANITARIAN PREPAID
	// MKC	NEBULA CONSUMER DEBIT SUPER PREMIUM
	// MEP	PREMIUM DEBIT MASTERCARD EMBOSSED
	// MGP	MASTERCARD PREPAID GOLD PAYROLL
	// TWB	WORLD BLACK EDITION IMMEDIATE DEBIT
	// MBC	MASTERCARD PREPAID VOUCHER
	// MVB	MASTERCARD B2B VIP 2
	// MVM	MASTERCARD B2B VIP 13
	// MVP	MASTERCARD B2B VIP 16
	// MVQ	MASTERCARD B2B VIP 17
	// MLL	MASTERCARD CENTRAL TRAVEL SOLUTIONS LAND
	// MVY	MASTERCARD B2B VIP 25
	// MRO	MASTERCARD REWARDS ONLY
	// MPP	PREPAID
	// MPD	MASTERCARD FLEX PREPAID
	// MVE	MASTERCARD B2B VIP 5
	// FIG	MASTERCARD B2B VIP 33
	// MKF	NEBULA CONSUMER CREDIT MASS AFFLUENT
	// TPC	MASTERCARD PROFESSIONAL CARD IMMEDIATE DEBIT
	// MVJ	MASTERCARD B2B VIP 10
	// MVO	MASTERCARD B2B VIP 15
	// MPJ	PREPAID MASTERCARD DEBIT VOUCHER MEAL FOOD CARD
	// MVF	MASTERCARD B2B VIP 6
	// MVA	MASTERCARD B2B VIP 1
	// MVC	MASTERCARD B2B VIP 3
	// MVD	MASTERCARD B2B VIP 4
	// MVL	MASTERCARD B2B VIP 12
	// MVR	MASTERCARD B2B VIP 18
	// MVS	MASTERCARD B2B VIP 19
	// MVT	MASTERCARD B2B VIP 20
	// MVU	MASTERCARD B2B VIP 21
	// MVV	MASTERCARD B2B VIP 22
	// MVW	MASTERCARD B2B VIP 23
	// MVZ	MASTERCARD B2B VIP 26
	// FIB	MASTERCARD B2B VIP 28
	// FIC	MASTERCARD B2B VIP 29
	// FIE	MASTERCARD B2B VIP 31
	// FIF	MASTERCARD B2B VIP 32
	// MVK	MASTERCARD B2B VIP 11
	// MVH	MASTERCARD B2B VIP 8
	// MVI	MASTERCARD B2B VIP 9
	// MWP	WORLD PREPAID TRAVEL
	// MES	MASTERCARD ENTERPRISE SOLUTION
	// MLE	MASTERCARD BRAZIL GENERAL BENEFITS
	// MLF	MASTERCARD AGRO
	// MSS	MAESTRO STUDENT CARD
	// MSO	PREPAID MAESTRO PREPAID OTHER CARD
	// PVH	PRIVATE LABEL HIPERCARD
	// MSW	PREPAID MAESTRO CORPORATE CARD
	// MSM	PREPAID MAESTRO CONSUMER PROMOTION CARD
}

func GetCardBrand(s string) string {
	// switch s {
	// case "MCC":
	// 	return "MasterCard® Credit"

	// case "DMC":
	// 	return "Debit MasterCard ®"

	// case "MSI":
	// 	return "Maestro ®"

	// case "CIR":
	// 	return "Cirrus ®"

	// case "PVL":
	// 	return "Private Label"

	// default:
	// 	return "Unknown"
	// }
	switch s {
	case "MCC":
		return "MasterCard"

	case "DMC":
		return "MasterCard"

	case "MSI":
		return "Maestro"

	case "CIR":
		return "Cirrus"

	case "PVL":
		return "Private Label"

	default:
		return "Unknown"
	}
}

func GetCardType(s string) string {
	switch s {
	case "MCC":
		return "Credit"

	case "DMC":
		return "Debit"

	case "MSI":
		return "Debit"

	case "CIR":
		return "Unknown"

	case "PVL":
		return "Unknown"

	default:
		return "Unknown"
	}
}

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
