package serve

// Flags holds all CLI flag values, mirroring curl's interface.
type Flags struct {
	// Request
	Method      string   // -X, --request
	Headers     []string // -H, --header
	Data        string   // -d, --data
	DataRaw     string   // --data-raw
	DataAscii   string   // --data-ascii
	DataBinary  string   // --data-binary
	DataURLEnc  string   // --data-urlencode
	JSONData    string   // --json
	FormItems   []string // -F, --form
	FormStrings []string // --form-string
	UploadFile  string   // -T, --upload-file

	// Auth
	User string // -u, --user

	// Output
	Output     string // -o, --output
	Silent     bool   // -s, --silent
	Verbose    bool   // -v, --verbose
	Include    bool   // -i, --include
	Head       bool   // -I, --head
	DumpHeader string // -D, --dump-header

	// Connection
	Proxy     string // -x, --proxy
	Insecure  bool   // -k, --insecure
	Timeout   int    // --max-time
	MaxRedirs int    // --max-redirs
	Location  bool   // -L, --location

	// Cookie
	Cookie    string // -b, --cookie
	CookieJar string // -c, --cookie-jar

	// TLS
	Cert string // --cert
	Key  string // --key

	// Misc
	Compressed bool   // --compressed
	UserAgent  string // -A, --user-agent
	Referer    string // -e, --referer
	RangeSpec  string // -r, --range
	ContinueAt string // -C, --continue-at
}
