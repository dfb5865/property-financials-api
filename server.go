package main

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/mux"
)

type PropertyData struct {
	Address string
	Price   float64
	HoaFee  float64
	Tax     float64
}

func formatPrice(price string) float64 {
	dollarAmountRegex := regexp.MustCompile(`^([-+] ?)?[0-9]+(,[0-9]+)?$`)
	trimmed := strings.Replace(strings.TrimLeft(strings.TrimSpace(price), "$"), ",", "", -1)
	match := dollarAmountRegex.FindStringSubmatch(trimmed)
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
	router.HandleFunc("/API/property", GetPropertyData).Queries("url", "")
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
		data.Price = formatPrice(text)
	})

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

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}

	w.Write(jsonData)
}
