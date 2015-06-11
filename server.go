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
	Address   string
	Price     float64
	HoaFee    float64
	Tax       float64
	Insurance int
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

func formatPrice(price string) float64 {
	//regex for price
	dollarAmountRegex := regexp.MustCompile(`^([-+] ?)?[0-9]+(,[0-9]+)?$`)

	trimmedWhiteSpace := strings.TrimSpace(price)
	stripDollars := strings.Replace(strings.TrimLeft(trimmedWhiteSpace, "$"), ",", "", -1)

	if strings.ToLower(stripDollars) == "off market" {
		return -1
	}

	match := dollarAmountRegex.FindStringSubmatch(stripDollars)

	if len(match) > 0 {
		i, err := strconv.ParseFloat(match[0], 64)
		if err != nil {
			panic(err)
		}
		return i
	}

	panic("Problem")
}

func main() {
	router := mux.NewRouter().StrictSlash(true)
	router.Headers("Access-Control-Allow-Origin: *")
	router.HandleFunc("/api/property", GetPropertyData).Queries("url", "")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func GetPropertyData(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	url := params.Get("url")

	doc, err := goquery.NewDocument(url)
	if err != nil {
		log.Fatal(err)
	}

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

	//Find HOA fees
	hoaRegex := regexp.MustCompile(`(?i)hoa fee: \$[0-9]*\/mo`)
	priceRegex := regexp.MustCompile(`\$[0-9]+(,[0-9]+)?`)

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
				data.Tax = formatPrice(price[0])
			}
		}
	})

	//https://homerates.honestpolicy.com/get_estimates?key=Zillow-hMy7jKq4fmM69782Q4m18&zip=12159&sqft=3000&est=438968&pid=29710887&year=1990&per=mo
	//use this api to get insurance estimates
	api := gopencils.Api("https://homerates.honestpolicy.com/")

	// Create a pointer to our response struct
	resp := &insuranceEstimateResponse{}

	zpid, _ := doc.Find("#zpidParam").Attr("value")
	zip := "92620" //fix this
	sqft := "3000" //fix this
	year := "2000" //fix this

	// Perform a GET request with Querystring
	querystring := map[string]string{"key": "Zillow-hMy7jKq4fmM69782Q4m18", "zip": zip, "sqft": sqft, "est": strconv.FormatFloat(data.Price, 'f', 6, 64), "pid": zpid, "year": year, "per": "mo"}

	_, err = api.Res("get_estimates", resp).Get(querystring)
	if err == nil {
		data.Insurance = resp.Rate
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	w.Write(jsonData)
}
