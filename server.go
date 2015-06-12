package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/bndr/gopencils"
	"github.com/gorilla/mux"
)

type PropertyData struct {
	Address          string  `json:"address"`
	Price            float64 `json:"purchasePrice"`
	MonthlyRent      float64 `json:"monthlyRent"`
	HoaFee           float64 `json:"monthlyHoa"`
	Tax              float64 `json:"monthlyTax"`
	Insurance        int     `json:"monthlyInsurance"`
	AppreciationRate float64 `json:"yearlyAppreciationRate"`
	Year             float64 `json:"yearBuilt"`
}

type insuranceEstimateResponse struct {
	Status    string
	Title     string
	Icon      string
	Link_text string
	Link_ref  string
	Rate      int
	Per       string
	Errors    []string
}

func main() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/api/property", GetPropertyData).Queries("url", "")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func formatPrice(price string) float64 {
	//regex for price
	numberRegex := regexp.MustCompile(`^([-+] ?)?[0-9]+(,[0-9]+)?$`)

	trimmedWhiteSpace := strings.TrimSpace(price)
	stripDollars := strings.Replace(strings.TrimLeft(trimmedWhiteSpace, "$"), ",", "", -1)

	if strings.ToLower(stripDollars) == "off market" {
		return -1
	}

	match := numberRegex.FindStringSubmatch(stripDollars)

	if len(match) > 0 {
		i, err := strconv.ParseFloat(match[0], 64)
		check(err)
		return i
	}

	return 0
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// fatal if there is an error
func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func GetPropertyData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Content-Type", "application/json")

	params := r.URL.Query()
	url := params.Get("url")

	doc, err := goquery.NewDocument(url)
	check(err)

	data := new(PropertyData)

	//Find address from zillow
	doc.Find(".zsg-content-header.addr h1").Each(func(_ int, node *goquery.Selection) {
		text := node.Text()
		data.Address = strings.TrimSpace(text)
	})

	//Find price from zillow
	doc.Find(".main-row.home-summary-row").Each(func(_ int, node *goquery.Selection) {
		text := node.Text()
		price := formatPrice(text)
		if price > 0 {
			data.Price = price
		}
	})

	//fall back to zestimate if there is no price
	if data.Price <= 0 {
		doc.Find(".estimates").Children().Each(func(i int, node *goquery.Selection) {
			if i == 1 {
				node.Find("span").Each(func(i int, spanNode *goquery.Selection) {
					if i == 1 {
						text := spanNode.Text()
						price := formatPrice(text)
						data.Price = price
					}
				})
			}
		})
	}

	//Find Rent Zestimate
	priceRegex := regexp.MustCompile(`\$[0-9]+(,[0-9]+)?`)
	doc.Find(".zest-title").Each(func(_ int, node *goquery.Selection) {
		text := node.Text()
		if strings.Contains(strings.ToLower(text), "rent") {
			rentText := node.Parent().Find(".zest-value").Text()
			price := priceRegex.FindStringSubmatch(rentText)
			if len(price) > 0 {
				data.MonthlyRent = formatPrice(price[0])
			}
		}
	})

	//Find HOA fees
	hoaRegex := regexp.MustCompile(`(?i)hoa fee: \$[0-9]*\/mo`)
	doc.Find(".fact-group-container.zsg-content-component.top-facts").Each(func(_ int, node *goquery.Selection) {
		text := node.Text()
		if hoaRegex.MatchString(text) {
			price := priceRegex.FindStringSubmatch(text)
			if len(price) > 0 {
				data.HoaFee = formatPrice(price[0])
			}
		}
	})

	//Find property tax
	doc.Find(".description.zsg-h4").Each(func(_ int, node *goquery.Selection) {
		text := node.Text()
		if text == "Property tax" {
			parent := node.Parent()
			cost := parent.Find(".vendor-cost").Text()
			price := priceRegex.FindStringSubmatch(cost)
			if len(price) > 0 {
				data.Tax = formatPrice(price[0]) / 12
			}
		}
	})

	//https://homerates.honestpolicy.com/get_estimates?key=Zillow-hMy7jKq4fmM69782Q4m18&zip=12159&sqft=3000&est=438968&pid=29710887&year=1990&per=mo
	//use this api to get insurance estimates
	api := gopencils.Api("https://homerates.honestpolicy.com/")

	// Create a pointer to our response struct
	resp := &insuranceEstimateResponse{}

	zpid, _ := doc.Find("#zpidParam").Attr("value")
	zip, _ := doc.Find("#loan-calculator-container").Attr("data-property-zipcode")
	sqft := ""
	doc.Find("span.addr_bbs").Each(func(_ int, node *goquery.Selection) {
		text := node.Text()
		sqftRegex := regexp.MustCompile(`^([-+] ?)?[0-9]+(,[0-9]+)? (sqft|square)$`)
		match := sqftRegex.FindStringSubmatch(text)
		if len(match) > 0 {
			numberRegex := regexp.MustCompile(`^([-+] ?)?[0-9]+(,[0-9]+)?`)
			sqft = strings.Replace(numberRegex.FindStringSubmatch(match[0])[0], ",", "", -1)
		}
	})
	year := ""
	doc.Find("div.hdp-facts .fact-group-container ul").Each(func(_ int, node *goquery.Selection) {
		text := node.Text()
		yearBuiltRegex := regexp.MustCompile(`(?i)built in \d{4}`)
		match := yearBuiltRegex.FindStringSubmatch(text)
		if len(match) > 0 {
			numberRegex := regexp.MustCompile(`\d{4}`)
			year = numberRegex.FindStringSubmatch(match[0])[0]
			data.Year, err = strconv.ParseFloat(year, 64)
			check(err)
		}
	})

	insurancePrice := strconv.FormatFloat(data.Price, 'f', 0, 64)

	// Perform a GET request with Querystring
	querystring := map[string]string{"key": "Zillow-hMy7jKq4fmM69782Q4m18", "zip": zip, "sqft": sqft, "est": insurancePrice, "pid": zpid, "year": year, "per": "mo"}

	_, err = api.Res("get_estimates", resp).Get(querystring)
	check(err)

	//Get neighborhood data (appreciation value)
	neighborhoodText := doc.Find("#hdp-neighborhood h4.zsg-content_collapsed").Parent().Text()
	matchSurroundingDecimal := regexp.MustCompile(`\s[a-zA-Z]+\s\d*.\d*%`)

	appreciationMatch := matchSurroundingDecimal.FindStringSubmatch(neighborhoodText)
	if len(appreciationMatch) > 0 {
		directionAndMagnitude := appreciationMatch[0]
		splitVector := strings.Split(directionAndMagnitude, " ")
		directionText := strings.ToLower(splitVector[1])
		magnitude, _ := strconv.ParseFloat(strings.Replace(splitVector[2], "%", "", -1), 64)

		positiveStrings := []string{"increased", "increase", "rise", "rose"}
		negativeStrings := []string{"fall", "fell", "decrease", "decreased"}

		if stringInSlice(directionText, positiveStrings) {
			data.AppreciationRate = magnitude
		}

		if stringInSlice(directionText, negativeStrings) {
			data.AppreciationRate = magnitude * -1
		}
	}

	jsonData, err := json.Marshal(data)
	check(err)

	w.Write(jsonData)
}
