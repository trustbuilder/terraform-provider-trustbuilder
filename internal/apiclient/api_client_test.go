package apiclient

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	apiClientServer             *http.Server
	apiClientTLSServer          *http.Server
	rootCA                      *x509.Certificate
	rootCAKey                   *ecdsa.PrivateKey
	serverCertPEM, serverKeyPEM []byte
	rootCAFilePath              = "rootCA.pem"
)

func TestCreateHashedJWT(t *testing.T) {
	tests := []struct {
		jwt      *JwtHashedToken
		expected string
	}{
		{
			&JwtHashedToken{
				Secret:     []byte("NotTheMostSecuredSecret"),
				Algortithm: "HS256",
				Claims:     map[string]any{"a": "b"},
			},
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhIjoiYiJ9.Y0CPYzbef6amCMdDiNVR6mV9ZUjES5Y3ynVRwaDqyh0",
		}, {
			&JwtHashedToken{
				Secret:     []byte("NotTheMostSecuredSecret"),
				Algortithm: "HS512",
				Claims:     map[string]any{"iss": "myIssuer", "sub": "mySubject", "aud": "myAudience", "nbf": "1758187630", "exp": "1758197630", "iat": "1758187630", "jti": "b46107a8-4de4-f1f3-2500-aa749dc01229"},
			},
			"eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJteUF1ZGllbmNlIiwiZXhwIjoiMTc1ODE5NzYzMCIsImlhdCI6IjE3NTgxODc2MzAiLCJpc3MiOiJteUlzc3VlciIsImp0aSI6ImI0NjEwN2E4LTRkZTQtZjFmMy0yNTAwLWFhNzQ5ZGMwMTIyOSIsIm5iZiI6IjE3NTgxODc2MzAiLCJzdWIiOiJteVN1YmplY3QifQ.JxSnALVbnIwMbSXsT9CXEsQkCuaixxfXlEQtzWier4E2uMF0-lGfoRvJ6dSjO9sLJ2mVhuc976ldlnpxWVsRaQ",
		},
	}

	for _, test := range tests {
		result, err := test.jwt.getSignedJwt()
		if err != nil {
			t.Errorf("createHashedJWT function returned an error: %s", err)
		}
		if result != test.expected {
			t.Errorf("createHashedJWT(jwt) = %s; want %s", result, test.expected)
		}
	}
}

func TestCreateHashedJWT_withDuration(t *testing.T) {
	jwtSecret := []byte("NotTheMostSecuredSecret")
	validityDurationMinute := int64(1)
	jwtHashedToken := &JwtHashedToken{
		Secret:                 jwtSecret,
		Algortithm:             "HS256",
		Claims:                 map[string]any{"iss": "myIssuer", "sub": "mySubject", "aud": "myAudience", "jti": "b46107a8-4de4-f1f3-2500-aa749dc01229"},
		ValidityDurationMinute: validityDurationMinute,
	}
	jwtHashedToken.completeClaimValidityTime()
	jwtString, err := jwtHashedToken.getSignedJwt()
	if err != nil {
		t.Errorf("The jwt signing returned the error: %s", err)
	}

	token, err := jwt.Parse(jwtString, func(token *jwt.Token) (any, error) {
		return jwtSecret, nil
	})
	if err != nil {
		t.Errorf("The jwt parsing returned the error: %s", err)
	}
	if !token.Valid {
		t.Errorf("Token not valid: %s", err)
	}

	//verify the validity duration
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		iat, err := claims.GetIssuedAt()
		if err != nil || iat == nil {
			t.Errorf("Error getting iat in claims: %s", err)
		}
		nbf, err := claims.GetNotBefore()
		if err != nil || nbf == nil {
			t.Errorf("Error getting nbf in claims: %s", err)
		}
		if *iat != *nbf {
			t.Error("In the claims, nbf and iat should be the same")
		}
		exp, err := claims.GetExpirationTime()
		if err != nil || exp == nil {
			t.Errorf("Error getting exp in claims: %s", err)
		}
		if exp.Sub(iat.Time) != (time.Duration(validityDurationMinute) * time.Minute) {
			t.Error("The difference between the claims exp and iat does not correspond to the validity duration")
		}
	} else {
		t.Error("Type assertion error on the claims")
	}
}

func TestAPIClient(t *testing.T) {
	debug := false

	if debug {
		log.Println("api_client_test.go: Starting HTTP server")
	}

	jwtSecret := []byte("NotTheMostSecuredSecret")
	setupAPIClientServer(jwtSecret)

	/* Notice the intentional trailing / */
	opt := &ApiClientOpt{
		Uri: "http://127.0.0.1:8083/",
		Jwt: &JwtHashedToken{
			Secret:                 jwtSecret,
			Algortithm:             "HS512",
			Claims:                 map[string]any{"iss": "myIssuer", "sub": "mySubject", "aud": "myAudience", "jti": "b46107a8-4de4-f1f3-2500-aa749dc01229"},
			ValidityDurationMinute: 1,
		},
		Insecure:            false,
		Username:            "",
		Password:            "",
		Headers:             make(map[string]string),
		Timeout:             2,
		IdAttribute:         "id",
		CopyKeys:            make([]string, 0),
		WriteReturnsObject:  false,
		CreateReturnsObject: false,
		RateLimit:           1,
		Debug:               debug,
	}
	client, _ := NewAPIClient(opt)

	var res string
	var err error

	if debug {
		log.Printf("api_client_test.go: Testing standard OK request\n")
	}
	res, err = client.SendRequest("GET", "/ok", "")
	if err != nil {
		t.Fatalf("api_client_test.go: %s", err)
	}
	if res != "It works!" {
		t.Fatalf("api_client_test.go: Got back '%s' but expected 'It works!'\n", res)
	}

	if debug {
		log.Printf("api_client_test.go: Testing redirect request\n")
	}
	res, err = client.SendRequest("GET", "/redirect", "")
	if err != nil {
		t.Fatalf("api_client_test.go: %s", err)
	}
	if res != "It works!" {
		t.Fatalf("api_client_test.go: Got back '%s' but expected 'It works!'\n", res)
	}

	/* Verify timeout works */
	if debug {
		log.Println("api_client_test.go: Testing timeout aborts requests")
	}
	_, err = client.SendRequest("GET", "/slow", "")
	if err == nil {
		t.Fatalf("api_client_test.go: Timeout did not trigger on slow request\n")
	}

	if debug {
		log.Println("api_client_test.go: Testing rate limited OK request")
	}
	startTime := time.Now().Unix()

	for i := 0; i < 4; i++ {
		_, err = client.SendRequest("GET", "/ok", "")
		if err != nil {
			t.Fatalf("api_client_test.go: Timeout did not trigger on ok request\n")
		}
	}

	if debug {
		log.Println("api_client_test.go: Testing jwt")
	}
	_, err = client.SendRequest("GET", "/validate-jwt", "")
	if err != nil {
		t.Fatalf("api_client_test.go: Error on JWT validation\n")
	}

	duration := time.Now().Unix() - startTime
	if duration < 3 {
		t.Fatalf("api_client_test.go: requests not delayed\n")
	}

	if debug {
		log.Println("api_client_test.go: Stopping HTTP server")
	}
	shutdownAPIClientServer()
	if debug {
		log.Println("api_client_test.go: Done")
	}

	// Setup and test HTTPS client with root CA
	setupAPIClientTLSServer()
	defer shutdownAPIClientTLSServer()
	defer os.Remove(rootCAFilePath)

	httpsOpt := &ApiClientOpt{
		Uri:                 "https://127.0.0.1:8443/",
		Jwt:                 nil,
		Insecure:            false,
		Username:            "",
		Password:            "",
		Headers:             make(map[string]string),
		Timeout:             2,
		IdAttribute:         "id",
		CopyKeys:            make([]string, 0),
		WriteReturnsObject:  false,
		CreateReturnsObject: false,
		RateLimit:           1,
		RootCaFile:          rootCAFilePath,
		Debug:               debug,
	}
	httpsClient, httpsClientErr := NewAPIClient(httpsOpt)

	if httpsClientErr != nil {
		t.Fatalf("api_client_test.go: %s", httpsClientErr)
	}
	if debug {
		log.Printf("api_client_test.go: Testing HTTPS standard OK request\n")
	}
	res, err = httpsClient.SendRequest("GET", "/ok", "")
	if err != nil {
		t.Fatalf("api_client_test.go: %s", err)
	}
	if res != "It works!" {
		t.Fatalf("api_client_test.go: Got back '%s' but expected 'It works!'\n", res)
	}
}

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("authorization header missing")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", fmt.Errorf("authorization header format must be Bearer {token}")
	}

	return strings.TrimPrefix(authHeader, prefix), nil
}

func setupAPIClientServer(jwtSecret []byte) {
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("It works!")); err != nil {
			log.Fatalf("api_client_test.go: Error on sending ok response: %s\n", err)
		}
	})
	serverMux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(9999 * time.Second)
		if _, err := w.Write([]byte("This will never return!!!!!")); err != nil {
			log.Fatalf("api_client_test.go: Error on sending slow response: %s\n", err)
		}
	})
	serverMux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusPermanentRedirect)
	})

	serverMux.HandleFunc("/validate-jwt", func(w http.ResponseWriter, r *http.Request) {
		debug := false
		if len(jwtSecret) != 0 {
			jwtTokenString, err := extractBearerToken(r)
			if debug {
				log.Printf("api_client_test.go: jwtTokenString content: %s", jwtTokenString)
			}
			if err != nil {
				log.Fatalf("api_client_test.go: Error on extracting the bearer token")
			}

			token, err := jwt.Parse(jwtTokenString, func(token *jwt.Token) (any, error) {
				return jwtSecret, nil
			})
			if err != nil {
				log.Fatalf("api_client_test.go: Error on parsing jwt: %s", err)
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				log.Fatalf("api_client_test.go: Error on MapClaims type assertion")
			}
			claimsJSON, err := json.Marshal(claims)
			if err != nil {
				log.Fatalf("api_client_test.go: Error marshalling claims: %s\n", err)
			}

			if _, err := w.Write([]byte(string(claimsJSON))); err != nil {
				log.Fatalf("api_client_test.go: Error on sending validate-jwt response: %s\n", err)
			}
		}
	})

	apiClientServer = &http.Server{
		Addr:    "127.0.0.1:8083",
		Handler: serverMux,
	}

	go func() {
		if err := apiClientServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("setupAPIClientServer(): %v", err)
		}
	}()
	/* let the server start */
	time.Sleep(1 * time.Second)
}

func shutdownAPIClientServer() {
	apiClientServer.Close()
}

func setupAPIClientTLSServer() {
	generateCertificates()

	cert, _ := tls.X509KeyPair(serverCertPEM, serverKeyPEM)

	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("It works!")); err != nil {
			log.Fatalf("setupAPIClientTLSServer(): %v", err)
		}
	})

	apiClientTLSServer = &http.Server{
		Addr:      "127.0.0.1:8443",
		Handler:   serverMux,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}

	go func() {
		if err := apiClientTLSServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("setupAPIClientTLSServer(): %v", err)
		}
	}()

	/* let the server start */
	time.Sleep(1 * time.Second)
}

func shutdownAPIClientTLSServer() {
	apiClientTLSServer.Close()
}

func generateCertificates() {
	// Create a CA certificate and key
	rootCAKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootCA = &x509.Certificate{
		SerialNumber: big.NewInt(2024),
		Subject: pkix.Name{
			Organization: []string{"Test Root CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}
	rootCABytes, _ := x509.CreateCertificate(rand.Reader, rootCA, rootCA, &rootCAKey.PublicKey, rootCAKey)

	// Create a server certificate and key
	serverKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serverCert := &x509.Certificate{
		SerialNumber: big.NewInt(2024),
		Subject: pkix.Name{
			Organization: []string{"Test Server"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 1),
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}

	// Add IP SANs to the server certificate
	serverCert.IPAddresses = append(serverCert.IPAddresses, net.ParseIP("127.0.0.1"))

	serverCertBytes, _ := x509.CreateCertificate(rand.Reader, serverCert, rootCA, &serverKey.PublicKey, rootCAKey)

	// PEM encode the certificates and keys
	serverCertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverCertBytes})

	// Marshal the server private key
	serverKeyBytes, _ := x509.MarshalECPrivateKey(serverKey)
	serverKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: serverKeyBytes})

	rootCAPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootCABytes})
	_ = os.WriteFile(rootCAFilePath, rootCAPEM, 0644)
}
