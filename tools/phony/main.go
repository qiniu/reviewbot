/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/qiniu/x/log"
)

var (
	address = flag.String("address", "http://localhost:8888/hook", "Where to send the fake hook.")
	hmacS   = flag.String("hmac", "abcde12345", "HMAC token to sign payload with.")
	event   = flag.String("event", "ping", "Type of event to send, such as pull_request.")
	payload = flag.String("payload", "", "File to send as payload. If unspecified, sends \"{}\".")
)

func main() {
	flag.Parse()

	var body []byte
	if *payload == "" {
		body = []byte("{}")
	} else {
		d, err := os.ReadFile(*payload)
		if err != nil {
			log.Fatal("Could not read payload file.", err)
		}
		body = d
	}

	if err := SendHook(*address, *event, body, []byte(*hmacS)); err != nil {
		log.Errorf("Error sending hook. err: %v", err)
	} else {
		log.Info("Hook sent.")
	}
}

// SendHook sends a GitHub event of type eventType to the provided address.
func SendHook(address, eventType string, payload, hmac []byte) error {
	req, err := http.NewRequest(http.MethodPost, address, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	req.Header.Set("X-GitHub-Event", eventType)
	req.Header.Set("X-GitHub-Delivery", "GUID")
	req.Header.Set("X-Hub-Signature", PayloadSignature(payload, hmac))
	req.Header.Set("content-type", "application/json")

	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("response from hook has status %d and body %s", resp.StatusCode, string(bytes.TrimSpace(rb)))
	}
	return nil
}

// PayloadSignature returns the signature that matches the payload.
func PayloadSignature(payload []byte, key []byte) string {
	mac := hmac.New(sha1.New, key)
	mac.Write(payload)
	sum := mac.Sum(nil)
	return "sha1=" + hex.EncodeToString(sum)
}
