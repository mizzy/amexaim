package main

import (
	"encoding/csv"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-querystring/query"
	gozaim "github.com/s-sasaki-0529/go-zaim"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type Payment struct {
	CategoryID    int    `url:"category_id"`
	GenreID       int    `url:"genre_id"`
	Amount        int    `url:"amount"`
	Date          string `url:"date"`
	FromAccountID int    `url:"from_account_id"`
	Comment       string `url:"comment"`
}

type Money struct {
	StartDate string `url:"start_date,omitempty"`
	EndData   string `url:"end_date,omitempty"`
	Mode      string `url:"mode,omitempty`
}

var zaim *gozaim.Client

func main() {
	zaim = gozaim.NewClient(
		os.Getenv("ZAIM_CONSUMER_ID"),
		os.Getenv("ZAIM_CONSUMER_SECRET"),
		os.Getenv("ZAIM_ACCESS_TOKEN"),
		os.Getenv("ZAIM_ACCESS_TOKEN_SECRET"),
	)

	accounts, err := zaim.FetchAccounts()
	if err != nil {
		log.Fatal(err)
	}

	var accountID int
	for _, account := range accounts {
		if account.Name == os.Getenv("ZAIM_ACCOUNT_NAME") {
			accountID = account.ID
		}
	}

	if accountID == 0 {
		log.Fatalf("Could not get account: %s", os.Getenv("ZAIM_ACCOUNT_NAME"))
	}

	categories, err := zaim.FetchCategories()
	if err != nil {
		log.Fatal(err)
	}

	var categoryID int
	for _, category := range categories {
		if category.Name == os.Getenv("ZAIM_CATEGORY") {
			categoryID = category.ID
		}
	}

	if categoryID == 0 {
		log.Fatalf("Could not get category: %s", os.Getenv("ZAIM_CATEGORY"))
	}

	genres, err := zaim.FetchGenres()
	if err != nil {
		log.Fatal(err)
	}

	var genreID int
	for _, genre := range genres {
		if genre.Name == os.Getenv("ZAIM_GENRE") {
			genreID = genre.ID
		}
	}

	if genreID == 0 {
		log.Fatalf("Could not get genre: %s", os.Getenv("ZAIM_GENRE"))
	}

	amex, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	reader := csv.NewReader(transform.NewReader(amex, japanese.ShiftJIS.NewDecoder()))
	reader.Read() // Skip header

	for {
		row, err := reader.Read()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		amount, _ := strconv.Atoi(strings.Replace(row[5], ",", "", -1))
		payment := Payment{
			CategoryID:    categoryID,
			GenreID:       genreID,
			Amount:        amount,
			Date:          strings.Replace(row[0], "/", "-", -1),
			FromAccountID: accountID,
			Comment:       strings.Replace(row[2], "?", "ー", -1),
		}

		duplicated, _ := payment.Duplicated()
		if duplicated {
			continue
		}

		params, _ := query.Values(payment)
		_, err = zaim.CreatePayment(params)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (p Payment) Duplicated() (bool, error) {
	money := Money{
		StartDate: p.Date,
		EndData:   p.Date,
		Mode:      "payment",
	}

	params, _ := query.Values(money)

	payments, err := zaim.FetchMoney(params)
	if err != nil {
		return false, err
	}

	for _, payment := range payments {
		if payment.FromAccountID != p.FromAccountID {
			continue
		}

		if payment.Amount == p.Amount && payment.Comment == p.Comment {
			return true, nil
		}
	}

	return false, nil
}
