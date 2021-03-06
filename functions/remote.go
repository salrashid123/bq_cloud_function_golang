package remote

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

type bqRequest struct {
	RequestId          string            `json:"requestId"`
	Caller             string            `json:"caller"`
	SessionUser        string            `json:"sessionUser"`
	UserDefinedContext map[string]string `json:"userDefinedContext"`
	Calls              [][]interface{}   `json:"calls"`
}

type bqResponse struct {
	Replies      []string `json:"replies,omitempty"`
	ErrorMessage string   `json:"errorMessage,omitempty"`
}

const ()

var ()

func init() {}

func HMAC_SHA256(w http.ResponseWriter, r *http.Request) {

	bqReq := &bqRequest{}
	bqResp := &bqResponse{}

	if err := json.NewDecoder(r.Body).Decode(&bqReq); err != nil {
		bqResp.ErrorMessage = fmt.Sprintf("External Function error: can't read POST body %v", err)
	} else {

		fmt.Printf("caller %s\n", bqReq.Caller)
		fmt.Printf("sessionUser %s\n", bqReq.SessionUser)
		fmt.Printf("userDefinedContext %v\n", bqReq.UserDefinedContext)

		wait := new(sync.WaitGroup)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		objs := make([]string, len(bqReq.Calls))

		for i, r := range bqReq.Calls {
			if len(r) != 2 {
				bqResp.ErrorMessage = fmt.Sprintf("Invalid number of input fields provided.  expected 2, got  %d", len(r))
			}
			raw, ok := r[0].(string)
			if !ok {
				bqResp.ErrorMessage = "Invalid plaintext type. expected string"
			}
			key, ok := r[1].(string)
			if !ok {
				bqResp.ErrorMessage = "Invalid key type. expected string"
			}
			if bqResp.ErrorMessage != "" {
				bqResp.Replies = nil
				break
			}
			//  use goroutines heres but keep the order
			wait.Add(1)
			go func(j int) {
				defer wait.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						h := hmac.New(sha256.New, []byte(key))
						_, err = io.WriteString(h, raw)
						if err != nil {
							bqResp.ErrorMessage = fmt.Sprintf("Error writing hmac  for row %d", j)
							bqResp.Replies = nil
							cancel()
							return
						}
						objs[j] = base64.StdEncoding.EncodeToString(h.Sum(nil))
						return
					}
				}
			}(i)
		}
		wait.Wait()
		if bqResp.ErrorMessage != "" {
			bqResp.Replies = nil
		} else {
			bqResp.Replies = objs
		}
	}

	b, err := json.Marshal(bqResp)
	if err != nil {
		http.Error(w, fmt.Sprintf("can't convert response to JSON %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
