package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Result struct {
	TrackingNum      string   `json:"trackingNum"`
	Carrier          Carrier  `json:"carrier"`
	Delivered        bool     `json:"delivered"`
	DeliveryDateTime string   `json:"deliveryDateTime,omitempty"` // parcel attempts to format the response as ISO 8601/RFC 3339 but this may be a dateTime string of an unknown format
	Updates          []Update `json:"updates,omitempty"`
}

type Update struct {
	DateTime string `json:"dateTime"` // parcel attempts to format the response as ISO 8601/RFC 3339 but this may be a dateTime string of an unknown format
	Location string `json:"location"`
	Status   string `json:"status"`
}

type Carrier string

const (
	DHL   Carrier = "DHL"
	FEDEX Carrier = "FEDEX"
	USPS  Carrier = "USPS"
	UPS   Carrier = "UPS"
)

const (
	URL        = "https://www.bing.com/packagetrackingv2?packNum=%s&carrier=%s"
	USER_AGENT = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36"
)

var TZ = time.Local

var (
	ErrArgs    = errors.New("too few arguments provided")
	ErrNum     = errors.New("invalid tracking number")
	ErrCarrier = errors.New("invalid carrier")
)

var (
	n      = flag.String("n", "", "tracking number [required]")
	c      = flag.String("c", "", "carrier [required]")
	o      = flag.String("o", "<stdout>", "path to output file")
	pretty = flag.Bool("pretty", false, "print the output json with indented fields")
	tz     = flag.String("tz", "", "the IANA time zone database location name to use when parsing date objects")
	g      = flag.Bool("gob", false, "encodes the output as a gob")
)

func main() {
	flag.Parse()
	if *n == "" || *c == "" {
		log.Println(ErrArgs.Error())
		flag.Usage()
		os.Exit(1)
	}

	num, err := SanitizeInput(*n)
	if err != nil {
		log.Fatalln(err.Error())
	}
	carrier, err := ValidateCarrier(*c)
	if err != nil {
		log.Fatalln(err.Error())
	}

	if *tz != "" {
		TZ, err = time.LoadLocation(*tz)
		if err != nil {
			log.Fatalln(err.Error())
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(URL, num, carrier), nil)
	if err != nil {
		cancel()
		log.Fatal(err.Error())
	}
	req.Header.Set("User-Agent", USER_AGENT)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		log.Fatal(err.Error())
	}

	res, err := Parse(resp.Body)
	resp.Body.Close()
	cancel()
	if err != nil {
		log.Fatalln(err.Error())
	}

	res.TrackingNum = num
	res.Carrier = carrier
	if len(res.Updates) == 0 {
		log.Println("tracking number updates not found")
	}

	// encode as gob and then exit
	if *g {
		err = EncodeGob(*o, res)
		if err != nil {
			log.Fatalln(err)
		}
		return
	}

	b := make([]byte, 0, 1024)
	switch *pretty {
	case true:
		b, err = json.MarshalIndent(res, "", "\t")
	case false:
		b, err = json.Marshal(res)
	}
	if err != nil {
		log.Fatalln(err.Error())
	}
	b = append(b, '\n')

	out, err := OutFile(*o)
	if err != nil {
		log.Fatalln(err.Error())
	}

	_, err = out.Write(b)
	if err != nil {
		out.Close()
		log.Fatalln(err.Error())
	}

	if err = out.Close(); err != nil {
		log.Fatalln(err.Error())
	}

}

func Parse(r io.Reader) (Result, error) {
	var res Result
	tokenizer := html.NewTokenizer(r)

	tmp := struct {
		Date, Time, Location, Status string
	}{}
	var i int
	for tType := tokenizer.Next(); tType != html.ErrorToken; tType = tokenizer.Next() {
		if tType != html.StartTagToken {
			continue
		}
		name, hasAttr := tokenizer.TagName()

		// parse most recent status and (estimated) delivery date
		if bytes.Equal(name, []byte("div")) && hasAttr {
			attr, val, _ := tokenizer.TagAttr()
			if bytes.Equal(attr, []byte("class")) && bytes.Equal(val, []byte("b_focusTextSmall")) {
				inner := tokenizer.Next()
				if inner == html.ErrorToken {
					break
				}
				if inner == html.TextToken {
					innterText := tokenizer.Text()
					b := bytes.Split(innterText, []byte(": "))
					if len(b) != 2 {
						continue
					}
					res.Delivered = bytes.Equal(b[0], []byte("Delivered"))
					if res.Delivered {
						res.DeliveryDateTime = ParseDeliveryDate(string(b[1]))
					} else {
						res.DeliveryDateTime = ParseEstimatedDelivery(string(b[1]))
					}

				}
			}
			continue
		}

		// parse updates
		if bytes.Equal(name, []byte("td")) {
			inner := tokenizer.Next()
			if inner == html.ErrorToken {
				break
			}
			if inner == html.TextToken {
				innerText := tokenizer.Text()
				switch i % 4 {
				case 0:
					tmp.Date = string(innerText)
				case 1:
					tmp.Time = string(innerText)
				case 2:
					tmp.Location = string(innerText)
				case 3:
					tmp.Status = string(innerText)
					res.Updates = append(res.Updates, Update{
						DateTime: ParseUpdateDateTime(tmp.Date, tmp.Time),
						Location: tmp.Location,
						Status:   tmp.Status,
					})
				}
				i++
			}
		}

	}
	if err := tokenizer.Err(); err != io.EOF {
		// there was an error parsing the input; this is most likely a context error
		return *new(Result), err

	}
	return res, nil
}

func SanitizeInput(s string) (string, error) {
	if len(s) < 7 || len(s) > 40 {
		return *new(string), ErrNum
	}
	out := new(strings.Builder)
	for _, char := range s {
		if (char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') {
			out.WriteRune(char)
		}
	}
	return out.String(), nil
}

func ValidateCarrier(s string) (Carrier, error) {
	switch Carrier(strings.ToUpper(s)) {
	case DHL:
		return DHL, nil
	case FEDEX:
		return FEDEX, nil
	case UPS:
		return UPS, nil
	case USPS:
		return USPS, nil
	}
	return *new(Carrier), ErrCarrier
}

func ParseUpdateDateTime(date, updateTime string) string {
	now := time.Now()
	if updateTime == "" {
		updateTime = "12:00 AM"
	}
	dt, err := time.ParseInLocation("Jan 2 3:04 PM 2006", date+" "+updateTime+" "+strconv.Itoa(now.Year()), TZ)
	if err != nil {
		// attempt to parse with year
		dt, err = time.ParseInLocation("Jan 2, 2006 3:04 PM", date+" "+updateTime, TZ)
		if err != nil {
			return date + ", " + updateTime
		}
		return dt.Format(time.RFC3339)
	}

	// Assuming all dates are within the current or preceding year
	if now.Before(dt) {
		dt = dt.AddDate(-1, 0, 0)
	}

	return dt.Format(time.RFC3339)
}

func ParseEstimatedDelivery(date string) string {
	dt, err := time.ParseInLocation("Monday, January 2, 2006", date, TZ)
	if err != nil {
		return date
	}
	return dt.Format(time.RFC3339)
}

func ParseDeliveryDate(date string) string {
	now := time.Now()

	// assume current year - this is kind of a hack, but avoids some of the messiness of manually
	// adding the current year after first parsing the (yearless) delivery date
	dt, err := time.ParseInLocation("Mon, Jan 02, 3:04 PM 2006", date+" "+strconv.Itoa(now.Year()), TZ)
	if err != nil {
		// if the first version doesn't work, try a second format
		dt, err := time.ParseInLocation("Mon, Jan 02, 2006, 3:04 PM", date, TZ)
		if err != nil {
			return date
		}
		return dt.Format(time.RFC3339)
	}

	// if the delivery date is in the future, assume the parcel was delivered in the prior year
	if now.Before(dt) {
		dt = dt.AddDate(-1, 0, 0)
	}
	return dt.Format(time.RFC3339)
}

func OutFile(s string) (*os.File, error) {
	if s == "<stdout>" {
		return os.Stdout, nil
	}
	return os.Create(s)
}
