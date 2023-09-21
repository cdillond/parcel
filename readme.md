## About
`parcel` is a simple CLI tool for tracking parcels. It makes use of a public and free, but unfortunately user-unfriendly API, for querying the statuses of parcels shipped by some common mail carriers. `parcel` currently supports tracking of packages shipped by USPS, UPS, FedEx, and DHL. **The entire existence of this program is kind of a workaround, so its reliability is not guaranteed, and it should not be used in production.**

## Installation
You can build `parcel` using Go by running:
```bash
go install github.com/cdillond/parcel@latest
```
## Usage
`parcel` takes a tracking number (specified by the `-n` flag) and a carrier (specified by the `-c` flag) as arguments and writes a JSON object to either `stdout` or the path specified by the `-o` option. If an output path is provided, a new file will be created; any existing file that shares the same path will be overwritten. If the `-pretty` option is specified, `parcel` indents the fields in the output JSON. The `-tz` option specifies the name of the location (as defined by the IANA time zone database) for `parcel` to use when parsing date-time data. To encode the output using gob instead of JSON, use the `-gob` flag.


Examples:
```bash 
$ parcel -n 1234567890 -c USPS -pretty
```
```bash 
$ parcel -n 1234567890 -c USPS -pretty -o out.json -tz "America/New_York"
```


The output takes the form:
 ```json
{
	"trackingNum": "1234567890",
	"carrier": "USPS",
	"delivered": true,
	"deliveryDateTime": "2023-09-19T14:51:00-04:00",
	"updates": [
		{
			"dateTime": "2023-09-19T14:51:00-04:00",
			"location": "Brooklyn, NY, United States",
			"status": "Delivered, in/at mailbox"
		},
		...
	]
}
```

## Notes
`parcel` attempts to format date objects as ISO 8601/RFC 3339 strings, but the `deliveryDateTime` and `dateTime` fields may contain strings of an undetermined format if `parcel` is unable to parse the response from the source API.

With the exception of `deliveryDateTime` values for parcels that have not yet been delivered, all dates are assumed to have occured within the current or prior year. `parcel` will produce inaccurate results for parcel shipments older than 1 year.

Additionally, all dates returned by the source API are parsed relative to the system's local time zone. If this does not match the time zone from which the request originates (e.g., due to the use of a proxy), the reported times will be incorrect. If this issue occurs, it can be rectified by specifying the correct time zone using the `-tz` flag.

The `updates` field contains a JSON array of no more than 5 elements. This field should therefore be considered as a list of only the most recent tracking updates, and may not provide a complete tracking history for a given parcel.



