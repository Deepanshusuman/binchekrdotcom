package helpers

import (
	"bufio"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func ProperTitle(input string) string {
	words := strings.Split(input, " ")
	smallwords := " a an on the to of and or in for "

	for index, word := range words {
		if strings.Contains(smallwords, " "+word+" ") && index != 0 {
			words[index] = word
		} else {
			words[index] = cases.Title(language.English).String(word)
		}
	}
	return strings.Join(words, " ")
}

func ParseNetwork(network string) string {
	//convert to lower case
	net := strings.ToLower(network)
	if strings.Contains(net, "rupay") {
		return "RuPay"
	} else if strings.Contains(net, "ebt") {
		return "EBT"
	} else if strings.Contains(net, "eftpos") {
		return "EFTPOS"
	} else if strings.Contains(net, "china union pay") {
		return "UnionPay"
	} else if strings.Contains(net, "uatp") {
		return "UATP"
	} else if strings.Contains(net, "nspk mir") {
		return "MIR"
	} else if strings.Contains(net, "jcb") {
		return "JCB"
	} else if strings.Contains(net, "prostir") {
		return "PROSTIR"
	} else if strings.Contains(net, "newday") {
		return "NewDay"
	} else if strings.Contains(net, "dinacard") {
		return "DinaCard"
	} else if strings.Contains(net, "argencard") {
		return "ArgenCard"
	} else if strings.Contains(net, "diners club international") {
		return "Diners Club International"
	}
	return network
}

func Getnull(s string) *string {
	if len(s) == 0 || strings.Contains(strings.ToLower(s), "null") {
		return nil
	}
	return &s
}

func Remove[T comparable](l []T, item T) []T {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
}
func Pad0(str string) string {
	switch len(str) {
	case 1:
		return str + "0000000000"
	case 2:
		return str + "000000000"
	case 3:
		return str + "00000000"
	case 4:
		return str + "0000000"
	case 5:
		return str + "000000"
	case 6:
		return str + "00000"
	case 7:
		return str + "0000"
	case 8:
		return str + "000"
	case 9:
		return str + "00"
	case 10:
		return str + "0"
	default:
		return str
	}
}

func Pad9(str string) string {
	switch len(str) {
	case 1:
		return str + "9999999999"
	case 2:
		return str + "999999999"
	case 3:
		return str + "99999999"
	case 4:
		return str + "9999999"
	case 5:
		return str + "999999"
	case 6:
		return str + "99999"
	case 7:
		return str + "9999"
	case 8:
		return str + "999"
	case 9:
		return str + "99"
	case 10:
		return str + "9"
	default:
		return str
	}

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

func IptoDecimal(ip string) int64 {
	IPv4Int := big.NewInt(0)
	IPv4Int.SetBytes(net.ParseIP(ip).To4())
	return IPv4Int.Int64()
}

func Image(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()
	fileInfo, _ := file.Stat()
	var size int64 = fileInfo.Size()
	bytes := make([]byte, size)
	buffer := bufio.NewReader(file)

	_, err = buffer.Read(bytes)
	if err != nil {
		fmt.Println(err)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func ImageSource(filename string) string {
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println(err)
	}

	return string(file)
}
