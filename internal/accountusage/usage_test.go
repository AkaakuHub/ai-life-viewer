package accountusage

import (
	"bytes"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestExchangeUsesOnlyInitializationAndRead(t *testing.T) {
	var sent bytes.Buffer
	response := strings.NewReader(`
{"jsonrpc":"2.0","id":1,"result":{}}
{"jsonrpc":"2.0","method":"ignored"}
{"jsonrpc":"2.0","id":2,"result":{"dailyUsageBuckets":[{"startDate":"2026-06-01","tokens":42}],"summary":{"lifetimeTokens":100}}}
`)
	snapshot, err := exchange(&sent, response)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.DailyUsageBuckets) != 1 || snapshot.DailyUsageBuckets[0].Tokens != 42 ||
		snapshot.Summary == nil || snapshot.Summary.LifetimeTokens != 100 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	dec := json.NewDecoder(&sent)
	var methods []string
	for {
		var request struct{ Method string }
		if err := dec.Decode(&request); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		methods = append(methods, request.Method)
	}
	if !slices.Equal(methods, []string{"initialize", "initialized", "account/usage/read"}) {
		t.Fatalf("unexpected RPC methods: %v", methods)
	}
}
