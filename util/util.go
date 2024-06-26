// Package util contains various general utility functions used throughout
// the project.
package util

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/go-playground/log"
)

// WrapError wraps error types to create compound error chains
func WrapError(text string, err error) error {
	return WrappedError{
		Text:  text,
		Inner: err,
	}
}

// WrappedError wraps error types to create compound error chains
type WrappedError struct {
	Text  string
	Inner error
}

func (e WrappedError) Error() string {
	text := e.Text
	if e.Inner != nil {
		text += ": " + e.Inner.Error()
	}
	return text
}

// Waterfall executes a slice of functions until the first error returned. This
// error, if any, is returned to the caller.
func Waterfall(fns ...func() error) (err error) {
	for _, fn := range fns {
		err = fn()
		if err != nil {
			break
		}
	}
	return
}

// Parallel executes functions in parallel. The first error is returned, if any.
func Parallel(fns ...func() error) error {
	ch := make(chan error)
	for i := range fns {
		fn := fns[i]
		go func() {
			ch <- fn()
		}()
	}

	for range fns {
		if err := <-ch; err != nil {
			return err
		}
	}

	return nil
}

// HashBuffer computes a base64 MD5 hash from a buffer
func HashBuffer(buf []byte) string {
	hash := md5.Sum(buf)
	return base64.RawStdEncoding.EncodeToString(hash[:])
}

// ConcatStrings efficiently concatenates strings with only one extra allocation
func ConcatStrings(s ...string) string {
	l := 0
	for _, s := range s {
		l += len(s)
	}
	b := make([]byte, 0, l)
	for _, s := range s {
		b = append(b, s...)
	}
	return string(b)
}

// CloneBytes creates a copy of b
func CloneBytes(b []byte) []byte {
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}

// SplitPunctuation splits off one byte of leading and trailing punctuation,
// if any, and returns the 3 split parts. If there is no edge punctuation, the
// respective byte = 0.
func SplitPunctuation(word []byte) (leading byte, mid []byte, trailing byte) {
	mid = word

	// Split leading
	if len(mid) < 2 {
		return
	}
	if isPunctuation(mid[0]) {
		leading = mid[0]
		mid = mid[1:]
	}

	// Split trailing
	l := len(mid)
	if l < 2 {
		return
	}
	if isPunctuation(mid[l-1]) {
		trailing = mid[l-1]
		mid = mid[:l-1]
	}

	return
}

// isPunctuation returns, if b is a punctuation symbol
func isPunctuation(b byte) bool {
	switch b {
	case '!', '"', '\'', '(', ')', ',', '-', '.', ':', ';', '?', '[', ']':
		return true
	default:
		return false
	}
}

// SplitPunctuationString splits off one byte of leading and trailing
// punctuation, if any, and returns the 3 split parts. If there is no edge
// punctuation, the respective byte = 0.
func SplitPunctuationString(word string) (
	leading byte, mid string, trailing byte,
) {
	// Generic copy paste :^)

	mid = word

	// Split leading
	if len(mid) < 2 {
		return
	}
	if isPunctuation(mid[0]) {
		leading = mid[0]
		mid = mid[1:]
	}

	// Split trailing
	l := len(mid)
	if l < 2 {
		return
	}
	if isPunctuation(mid[l-1]) {
		trailing = mid[l-1]
		mid = mid[:l-1]
	}

	return
}

type VerificationResponse struct {
	Header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	} `json:"header"`
	Payload struct {
		Iss     string `json:"iss"`
		Sub     string `json:"sub"`
		Aud     string `json:"aud"`
		Exp     string `json:"exp"`
		Nbf     int    `json:"nbf"`
		Iat     int    `json:"iat"`
		Jti     string `json:"jti"`
		Address string `json:"address"`
		Data    struct {
			Milady struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"milady"`
			Remilio struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"remilio"`
			Yayo struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"yayo"`
			Kagami struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"kagami"`
			Fruits struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"fruits"`
			Banners struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"banners"`
			Bitch struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"bitch"`
			Bonkler struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"bonkler"`
			Pixelady struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"pixelady"`
			GodsRemix struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"gods_remix"`
			Fumo struct {
				Type string `json:"type"`
				Hex  string `json:"hex"`
			} `json:"fumo"`
			Admin     bool `json:"admin"`
			CultBuyer bool `json:"cultBuyer"`
		} `json:"data"`
	} `json:"payload"`
}

func ConnectedWalletAddress(r *http.Request) (address string, jwt string) {
	address = "none"
	jwt = "none"
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://gate.miladychan.org/verify", nil)
	if err != nil {
		return
	}

	req.Header = r.Header
	resp, err := client.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Errorf("Client Wallet Auth: Failed to verify wallet owner")
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var response VerificationResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Errorf("Client Wallet Auth: Failed to unmarshal response")
		return
	}

	jwt = r.Header.Get("authToken")
	address = response.Payload.Address
	log.Infof("Client Wallet Auth: Verfied Owner")
	defer resp.Body.Close()
	return
}
