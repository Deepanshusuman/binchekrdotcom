package helpers

import (
	"strconv"
)

type digits [8]int

func (d *digits) at(i int) int {
	return d[i-1]
}

func VerifyCard(s string) (cardType CardType) {
	cardType = determineCardType(s)
	for _, length := range cardType.Lengths {
		if len(s) == int(length) {
			cardType.Valid = true
			break
		}
	}
	cardType.Valid = validateLuhn(s)
	return
}
func determineCardType(c string) CardType {
	ccLen := len(c)
	ccDigits := digits{}
	for i := 0; i < 8; i++ {
		if i < ccLen {
			ccDigits[i], _ = strconv.Atoi(c[:i+1])
		}
	}
	one := ccDigits.at(1)
	two := ccDigits.at(2)
	three := ccDigits.at(3)
	four := ccDigits.at(4)
	six := ccDigits.at(6)
	eight := ccDigits.at(8)
	switch {
	case six == 401178 ||
		six == 401179 ||
		six == 438935 ||
		six == 457631 ||
		six == 457632 ||
		six == 431274 ||
		six == 451416 ||
		six == 457393 ||
		six == 504175 ||
		six == 627780 ||
		six == 636297 ||
		six == 636368 ||
		(six >= 506699 && six <= 506778) ||
		(six >= 509000 && six <= 509999) ||
		(six >= 650031 && six <= 650033) ||
		(six >= 650035 && six <= 650051) ||
		(six >= 650405 && six <= 650439) ||
		(six >= 650485 && six <= 650538) ||
		(six >= 650541 && six <= 650598) ||
		(six >= 650700 && six <= 650718) ||
		(six >= 650720 && six <= 650727) ||
		(six >= 650901 && six <= 650978) ||
		(six >= 651652 && six <= 651679) ||
		(six >= 655000 && six <= 655019) ||
		(six >= 655021 && six <= 655058):
		return CardType{
			NiceNetwork: "Elo",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16},
			CodeName:    "CVE",
			CodeSize:    3,
		}

	case (four >= 2200 && four <= 2204):
		return CardType{
			NiceNetwork: "Mir",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16, 17, 18, 19},
			CodeName:    "CVP2",
			CodeSize:    3,
		}

	case six == 637095 || eight == 63737423 || eight == 63743358 ||
		six == 637568 || six == 637599 || six == 637609 ||
		six == 637612:
		return CardType{
			NiceNetwork: "Hiper",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16},
			CodeName:    "CVC",
			CodeSize:    3,
		}
	case six == 606282:
		return CardType{
			NiceNetwork: "Hipercard",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16},
			CodeName:    "CVC",
			CodeSize:    3,
		}
	case two == 34 || two == 37:
		return CardType{
			NiceNetwork: "American Express",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{15},
			CodeName:    "CID",
			CodeSize:    4,
		}

	case (six >= 622126 && six <= 623796) || (six >= 624000 && six <= 626999) || (six >= 628200 && six <= 628899) || (six >= 810000 && six <= 817199):
		return CardType{
			NiceNetwork: "UnionPay",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16, 17, 18, 19},
			CodeName:    "CVN",
			CodeSize:    3,
		}

	case (three >= 300 && three <= 305) || three == 309 ||
		two == 36 || two == 38 || two == 39:
		return CardType{
			NiceNetwork: "Diners Club",
			Gaps:        []int32{4, 10},
			Lengths:     []int32{14, 16, 19},
			CodeName:    "CVC",
			CodeSize:    3,
		}

	case four == 6011 || (three >= 644 && three <= 649) || two == 65:
		return CardType{
			NiceNetwork: "Discover",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16, 19},
			CodeName:    "CID",
			CodeSize:    3,
		}

	case (two >= 51 && two <= 55) ||
		(four >= 2221 && four <= 2720):
		return CardType{
			NiceNetwork: "Mastercard",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16, 19},
			CodeName:    "CVC",
			CodeSize:    3,
		}

	case four == 2131 || four == 1800 || (four >= 3528 && four <= 3589):
		return CardType{
			NiceNetwork: "JCB",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16, 17, 18, 19},
			CodeName:    "CVV",
			CodeSize:    3,
		}

	case (six >= 639000 && six <= 639099) ||
		(six >= 670000 && six <= 679999):

		return CardType{
			NiceNetwork: "Maestro",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{12, 13, 14, 15, 16, 17, 18, 19},
			CodeName:    "CVC",
			CodeSize:    3,
		}
	case one == 4:
		return CardType{
			NiceNetwork: "Visa",
			Gaps:        []int32{4, 8, 12},
			Lengths:     []int32{16, 19},
			CodeName:    "CVV",
			CodeSize:    3,
		}
	case one == 1:
		return CardType{
			NiceNetwork: "UATP",
			Gaps:        []int32{4, 9, 15},
			Lengths:     []int32{15},
			CodeName:    "CVC",
			CodeSize:    0,
		}
	default:
		return CardType{
			NiceNetwork: "",
			Gaps:        nil,
			Lengths:     nil,
			CodeName:    "",
			CodeSize:    0,
		}
	}

}
func validateLuhn(c string) bool {
	var sum int
	var alternate bool
	numberLen := len(c)

	if numberLen < 13 || numberLen > 19 {
		return false
	}

	for i := numberLen - 1; i > -1; i-- {
		mod, _ := strconv.Atoi(string(c[i]))
		if alternate {
			mod *= 2
			if mod > 9 {
				mod = (mod % 10) + 1
			}
		}

		alternate = !alternate
		sum += mod
	}

	return sum%10 == 0
}
