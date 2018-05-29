// Print account info for given address.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/laxmicoinofficial/go/clients/orbit"
)

var (
	orbitDomainFlag = flag.String("orbit", "http://localhost:8000", "orbit address")
	addressFlag       = flag.String("address", "", "account address")
)

func main() {
	flag.Parse()

	client := orbit.Client{
		URL:  *horizonDomainFlag,
		HTTP: &http.Client{Timeout: 5 * time.Second},
	}

	account, err := client.LoadAccount(*addressFlag)
	if err != nil {
		panic(err)
	}

	b, err := json.Marshal(&account)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(b))
}
