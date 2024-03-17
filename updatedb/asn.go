package main

import (
	"strconv"
	"strings"

	"github.com/likexian/whois"
)

type AsnInfo struct {
	ASNumber      int64
	ASName        string
	OrgName       string
	OrgId         string
	OrgAbuseEmail string
	OrgAbusePhone string
}

func Asn(asn string) (AsnInfo, error) {
	result, err := whois.Whois(asn)
	as := AsnInfo{}
	if err == nil {
		for _, v := range strings.Split(result, "\n") {
			if len(v) == 0 || string(v[0]) == "#" || string(v[0]) == "%" {
				continue
			}
			info := strings.Split(strings.ReplaceAll(v, " ", ""), ":")
			if info[0] == "ASNumber" {
				as.ASNumber, _ = strconv.ParseInt(info[1], 10, 64)
			}

			if info[0] == "ASName" {
				as.ASName = info[1]
			}

			if info[0] == "OrgName" {
				as.OrgName = info[1]
			}

			if info[0] == "OrgId" {
				as.OrgId = info[1]
			}

			if info[0] == "OrgAbuseEmail" {
				as.OrgAbuseEmail = info[1]
			}

			if info[0] == "OrgAbusePhone" {
				as.OrgAbusePhone = info[1]
			}
		}
	}
	return as, err
}
