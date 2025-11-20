package apiclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"golang.org/x/time/rate"

	jwtgen "github.com/golang-jwt/jwt/v5"
)

type JwtHashedToken struct {
	Secret                 []byte
	Algortithm             string
	Claims                 map[string]any
	ValidityDurationMinute int64
}

type ApiClientOpt struct {
	Uri                 string
	Jwt                 *JwtHashedToken
	Insecure            bool
	Username            string
	Password            string
	Headers             map[string]string
	Timeout             int64
	IdAttribute         string
	CreateMethod        string
	ReadMethod          string
	ReadData            string
	UpdateMethod        string
	UpdateData          string
	DestroyMethod       string
	DestroyData         string
	CopyKeys            []string
	WriteReturnsObject  bool
	CreateReturnsObject bool
	XssiPrefix          string
	UseCookies          bool
	RateLimit           float64
	OauthClientID       string
	OauthClientSecret   string
	OauthScopes         []string
	OauthTokenURL       string
	OauthEndpointParams url.Values
	CertFile            string
	KeyFile             string
	RootCaFile          string
	CertString          string
	KeyString           string
	RootCaString        string
	Debug               bool
}

/*APIClient is a HTTP client with additional controlling fields.*/
type APIClient struct {
	HttpClient          *http.Client
	Uri                 string
	Jwt                 *JwtHashedToken
	Insecure            bool
	Username            string
	Password            string
	Headers             map[string]string
	IdAttribute         string
	CreateMethod        string
	ReadMethod          string
	ReadData            string
	UpdateMethod        string
	UpdateData          string
	DestroyMethod       string
	DestroyData         string
	CopyKeys            []string
	WriteReturnsObject  bool
	CreateReturnsObject bool
	XssiPrefix          string
	RateLimiter         *rate.Limiter
	Debug               bool
	OauthConfig         *clientcredentials.Config
}

func (jwt *JwtHashedToken) completeClaimValidityTime() {
	if jwt.ValidityDurationMinute > 0 {
		epoch := time.Now().Unix()
		jwt.Claims["nbf"] = epoch
		jwt.Claims["iat"] = epoch
		jwt.Claims["exp"] = epoch + (jwt.ValidityDurationMinute * 60)
	}
}

func (jwt *JwtHashedToken) getSignedJwt() (string, error) {
	signer := jwtgen.GetSigningMethod(jwt.Algortithm)
	token := jwtgen.NewWithClaims(signer, jwtgen.MapClaims(jwt.Claims))

	return token.SignedString(jwt.Secret)
}

// Returns a map from the given string in JSON format.
// If the JSON is an array, only the first object is converted.
func JsonDecodeApiResponse(jsonData string) (map[string]any, error) {
	var data any
	var mapData map[string]any
	var ok bool

	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, err
	}

	switch v := data.(type) {
	case map[string]any:
		mapData, ok = data.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("type assertion from any to map[string]any failed")
		}
	case []any:
		if array, ok := data.([]any); !ok {
			return nil, fmt.Errorf("type assertion from any to []any failed")
		} else if len(array) > 1 {
			return nil, fmt.Errorf("unmarshalApiResponse() can't manage a JSON with an array length > 1")
		}
		mapData, ok = data.([]any)[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("type assertion to map[string]any of the first array element failed")
		}
	default:
		return nil, fmt.Errorf("the json data is not an array, neither a map: %T", v)
	}

	return mapData, nil
}

func JsonEncode(data map[string]any) (string, error) {

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("the data can't be encoded into JSON: %v", data)
	}
	return string(jsonBytes), err
}

// If the value of the key is not a string, returns an error.
func GetKeyValue(jsonData string, key string) (string, error) {
	var ok bool
	var value any
	var result string

	mapData, err := JsonDecodeApiResponse(jsonData)
	if err != nil {
		return "", err
	}
	value, ok = mapData[key]
	if !ok {
		return "", fmt.Errorf("key %s not found", key)
	}
	result, ok = value.(string)
	if !ok {
		return "", fmt.Errorf("the value of the key %s can't be casted into string: %v", key, value)
	}

	return result, nil
}

// NewAPIClient makes a new api client for RESTful calls.
func NewAPIClient(opt *ApiClientOpt) (*APIClient, error) {
	if opt.Debug {
		log.Printf("api_client.go: Constructing debug api_client\n")
	}

	if opt.Uri == "" {
		return nil, errors.New("uri must be set to construct an API client")
	}

	/* Sane default */
	if opt.IdAttribute == "" {
		opt.IdAttribute = "id"
	}

	/* Remove any trailing slashes since we will append
	   to this URL with our own root-prefixed location */
	opt.Uri = strings.TrimSuffix(opt.Uri, "/")

	if opt.CreateMethod == "" {
		opt.CreateMethod = "POST"
	}
	if opt.ReadMethod == "" {
		opt.ReadMethod = "GET"
	}
	if opt.UpdateMethod == "" {
		opt.UpdateMethod = "PUT"
	}
	if opt.DestroyMethod == "" {
		opt.DestroyMethod = "DELETE"
	}

	tlsConfig := &tls.Config{
		/* Disable TLS verification if requested */
		InsecureSkipVerify: opt.Insecure,
	}

	if opt.CertString != "" && opt.KeyString != "" {
		cert, err := tls.X509KeyPair([]byte(opt.CertString), []byte(opt.KeyString))
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if opt.CertFile != "" && opt.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(opt.CertFile, opt.KeyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load root CA
	if opt.RootCaFile != "" || opt.RootCaString != "" {
		caCertPool := x509.NewCertPool()
		var rootCA []byte
		var err error

		if opt.RootCaFile != "" {
			if opt.Debug {
				log.Printf("api_client.go: Reading root CA file: %s\n", opt.RootCaFile)
			}
			rootCA, err = os.ReadFile(opt.RootCaFile)
			if err != nil {
				return nil, fmt.Errorf("could not read root CA file: %v", err)
			}
		} else {
			if opt.Debug {
				log.Printf("api_client.go: Using provided root CA string\n")
			}
			rootCA = []byte(opt.RootCaString)
		}

		if !caCertPool.AppendCertsFromPEM(rootCA) {
			return nil, errors.New("failed to append root CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	var cookieJar http.CookieJar

	if opt.UseCookies {
		cookieJar, _ = cookiejar.New(nil)
	}

	rateLimit := rate.Limit(opt.RateLimit)
	bucketSize := int(math.Max(math.Round(opt.RateLimit), 1))
	log.Printf("limit: %f bucket: %d", opt.RateLimit, bucketSize)
	rateLimiter := rate.NewLimiter(rateLimit, bucketSize)

	client := APIClient{
		HttpClient: &http.Client{
			Timeout:   time.Second * time.Duration(opt.Timeout),
			Transport: tr,
			Jar:       cookieJar,
		},
		RateLimiter:         rateLimiter,
		Uri:                 opt.Uri,
		Jwt:                 opt.Jwt,
		Insecure:            opt.Insecure,
		Username:            opt.Username,
		Password:            opt.Password,
		Headers:             opt.Headers,
		IdAttribute:         opt.IdAttribute,
		CreateMethod:        opt.CreateMethod,
		ReadMethod:          opt.ReadMethod,
		ReadData:            opt.ReadData,
		UpdateMethod:        opt.UpdateMethod,
		UpdateData:          opt.UpdateData,
		DestroyMethod:       opt.DestroyMethod,
		DestroyData:         opt.DestroyData,
		CopyKeys:            opt.CopyKeys,
		WriteReturnsObject:  opt.WriteReturnsObject,
		CreateReturnsObject: opt.CreateReturnsObject,
		XssiPrefix:          opt.XssiPrefix,
		Debug:               opt.Debug,
	}

	if opt.OauthClientID != "" && opt.OauthClientSecret != "" && opt.OauthTokenURL != "" {
		client.OauthConfig = &clientcredentials.Config{
			ClientID:       opt.OauthClientID,
			ClientSecret:   opt.OauthClientSecret,
			TokenURL:       opt.OauthTokenURL,
			Scopes:         opt.OauthScopes,
			EndpointParams: opt.OauthEndpointParams,
		}
	}

	if opt.Debug {
		log.Printf("api_client.go: Constructed client:\n%s", client.toString())
	}
	return &client, nil
}

// Convert the important bits about this object to string representation
// This is useful for debugging.
func (client *APIClient) toString() string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("uri: %s\n", client.Uri))
	if client.Jwt != nil {
		buffer.WriteString(fmt.Sprintf("jwt_hashed_token.secret: %s\n", client.Jwt.Secret))
		buffer.WriteString(fmt.Sprintf("jwt_hashed_token.algorithm: %s\n", client.Jwt.Algortithm))
		buffer.WriteString(fmt.Sprintf("jwt_hashed_token.claimsJson: %s\n", client.Jwt.Claims))
	}
	buffer.WriteString(fmt.Sprintf("insecure: %t\n", client.Insecure))
	buffer.WriteString(fmt.Sprintf("username: %s\n", client.Username))
	buffer.WriteString(fmt.Sprintf("password: %s\n", client.Password))
	buffer.WriteString(fmt.Sprintf("id_attribute: %s\n", client.IdAttribute))
	buffer.WriteString(fmt.Sprintf("write_returns_object: %t\n", client.WriteReturnsObject))
	buffer.WriteString(fmt.Sprintf("create_returns_object: %t\n", client.CreateReturnsObject))
	buffer.WriteString("headers:\n")
	for k, v := range client.Headers {
		buffer.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
	}
	for _, n := range client.CopyKeys {
		buffer.WriteString(fmt.Sprintf("  %s", n))
	}
	return buffer.String()
}

/*
Helper function that handles sending/receiving and handling

	of HTTP data in and out.
*/
func (client *APIClient) SendRequest(method string, path string, data string) (string, error) {
	fullURI := client.Uri + path
	var req *http.Request
	var err error

	if client.Debug {
		log.Printf("api_client.go: method=%s, path=%s, full uri (derived)=%s, data=%s\n", method, path, fullURI, data)
	}

	buffer := bytes.NewBuffer([]byte(data))

	if data == "" {
		req, err = http.NewRequest(method, fullURI, nil)
	} else {
		req, err = http.NewRequest(method, fullURI, buffer)

		/* Default of application/json, but allow headers array to overwrite later */
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	if err != nil {
		log.Fatal(err)
		return "", err
	}

	if client.Debug {
		log.Printf("api_client.go: Sending HTTP request to %s...\n", req.URL)
	}

	/* Allow for tokens or other pre-created secrets */
	if len(client.Headers) > 0 {
		for n, v := range client.Headers {
			req.Header.Set(n, v)
		}
	}

	if client.Jwt != nil {
		client.Jwt.completeClaimValidityTime()
		jwt, _ := client.Jwt.getSignedJwt()
		req.Header.Set("Authorization", "Bearer "+jwt)
	}

	if client.OauthConfig != nil {
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client.HttpClient)
		tokenSource := client.OauthConfig.TokenSource(ctx)
		token, err := tokenSource.Token()
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	}

	if client.Username != "" && client.Password != "" {
		/* ... and fall back to basic auth if configured */
		req.SetBasicAuth(client.Username, client.Password)
	}

	if client.Debug {
		log.Printf("api_client.go: Request headers:\n")
		for name, headers := range req.Header {
			for _, h := range headers {
				log.Printf("api_client.go:   %v: %v", name, h)
			}
		}

		log.Printf("api_client.go: BODY:\n")
		body := "<none>"
		if req.Body != nil {
			body = data
		}
		log.Printf("%s\n", body)
	}

	if client.RateLimiter != nil {
		// Rate limiting
		if client.Debug {
			log.Printf("Waiting for rate limit availability\n")
		}
		_ = client.RateLimiter.Wait(context.Background())
	}

	resp, err := client.HttpClient.Do(req)

	if err != nil {
		//log.Printf("api_client.go: Error detected: %s\n", err)
		return "", err
	}

	if client.Debug {
		log.Printf("api_client.go: Response code: %d\n", resp.StatusCode)
		log.Printf("api_client.go: Response headers:\n")
		for name, headers := range resp.Header {
			for _, h := range headers {
				log.Printf("api_client.go:   %v: %v", name, h)
			}
		}
	}

	bodyBytes, err2 := io.ReadAll(resp.Body)
	resp.Body.Close()

	if err2 != nil {
		return "", err2
	}
	body := strings.TrimPrefix(string(bodyBytes), client.XssiPrefix)
	if client.Debug {
		log.Printf("api_client.go: BODY:\n%s\n", body)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return body, fmt.Errorf("unexpected response code '%d': %s", resp.StatusCode, body)
	}

	if body == "" {
		return "{}", nil
	}

	return body, nil

}
