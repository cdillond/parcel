## About
`parcel` is a simple CLI tool for tracking parcels. It makes use of a public and free, but unfortunately user-unfriendly API, for querying the statuses of parcels shipped by some common mail carriers. `parcel` currently supports tracking of packages shipped by USPS, UPS, FedEx, and DHL. The entire existence of this program is kind of a workaround, so its reliability is not guaranteed, and it should not be used in production.

## Installation
You can build `parcel` using Go by running:
```bash
$ go install https://github.com/cdillond/parcel@latest
```
## Usage
`parcel` takes a tracking number (specified by the `-n` flag) and a carrier (specified by the `-c` flag) as arguments and writes a JSON object to either `stdout` or the path specified by the `-o` option. If an output path is provided, a new file will be created; any existing file that shares the same path will be overwritten.


Examples:
```bash 
$ parcel -n 1234567891234561234123412348 -c USPS
```
```bash 
$ parcel -n 1234567891234561234123412348 -c USPS -o out.json
```


The output takes the form:
 ```json
{
    "trackingNum":"1234567891234561234123412348",
    "carrier":"USPS",
    "delivered":true,
    "deliveryDateTime":"2023-09-15T10:14:00Z", 
    "updates":[
            {
            "dateTime":"2023-09-15T10:14:00Z",
            "location":"Brooklyn, NY, United States",
            "status":"Delivered"
            },
            ...
        ]
}

```
`parcel` attempts to format date objects as ISO 8601/RFC 3339 strings, but the `deliveryDateTime` and `dateTime` fields may contain strings of an undetermined format if `parcel` is unable to parse the response from the source API.


