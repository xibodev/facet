// Deterministic CLI protocol fixture, not an AI engine or acceptance evidence.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	args := strings.Join(os.Args[1:], " ")
	id := "fixture-native-conversation"
	f, err := os.OpenFile("fixture-turns.jsonl", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	_ = json.NewEncoder(f).Encode(map[string]string{"args": args})
	_ = f.Close()
	fmt.Printf("{\"type\":\"thread.started\",\"thread_id\":%q}\n", id)
	fmt.Println(`{"type":"item.completed","item":{"type":"agent_message","text":"Deterministic fixture response. Not AI acceptance."}}`)
	fmt.Println(`{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}`)
}
